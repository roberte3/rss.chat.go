package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// resolveHandle asks resolverBase (an AT Protocol entryway, e.g.
// https://bsky.social) to resolve a handle to its DID. This is a
// network-wide directory lookup, not specific to any one PDS.
func resolveHandle(ctx context.Context, client *http.Client, resolverBase, handle string) (string, error) {
	reqURL := strings.TrimRight(resolverBase, "/") +
		"/xrpc/com.atproto.identity.resolveHandle?handle=" + url.QueryEscape(handle)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("resolveHandle(%s): status %d", handle, resp.StatusCode)
	}

	var out struct {
		DID string `json:"did"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.DID == "" {
		return "", fmt.Errorf("resolveHandle(%s): empty did in response", handle)
	}
	return out.DID, nil
}

type didDocument struct {
	Service []struct {
		ID              string `json:"id"`
		Type            string `json:"type"`
		ServiceEndpoint string `json:"serviceEndpoint"`
	} `json:"service"`
}

// resolvePDSEndpoint finds a DID's Personal Data Server so listRecords can be
// called directly against it, rather than an AppView aggregation endpoint.
// did:plc documents come from the PLC directory; did:web documents come from
// the domain's own /.well-known/did.json, per the did:web spec.
func resolvePDSEndpoint(ctx context.Context, client *http.Client, plcDirectoryBase, did string) (string, error) {
	var docURL string
	if strings.HasPrefix(did, "did:web:") {
		domain := strings.TrimPrefix(did, "did:web:")
		parts := strings.Split(domain, ":")
		host := strings.ReplaceAll(parts[0], "%3A", ":") // did:web encodes a port as %3A
		path := ""
		if len(parts) > 1 {
			path = "/" + strings.Join(parts[1:], "/")
		}
		docURL = "https://" + host + path + "/.well-known/did.json"
	} else {
		docURL = strings.TrimRight(plcDirectoryBase, "/") + "/" + url.PathEscape(did)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, docURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("resolving PDS for %s: status %d", did, resp.StatusCode)
	}

	var doc didDocument
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return "", err
	}
	for _, svc := range doc.Service {
		if svc.Type == "AtprotoPersonalDataServer" {
			return svc.ServiceEndpoint, nil
		}
	}
	return "", fmt.Errorf("no AtprotoPersonalDataServer service in DID document for %s", did)
}
