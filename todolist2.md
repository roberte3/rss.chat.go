 Based on the changes I can see in the git diff, here are the API changes that
 Dave Winer has made to the original RSS.Chat since the last update:

 API Changes Identified

 1. New API Endpoint: /readhttpfile
     - Added a new endpoint that allows reading HTTP files (used for the
       Scripts menu functionality)
     - This endpoint was previously implemented inline but now uses a
       dedicated handler function
     - It's designed to fetch OPML files for the Scripts menu functionality

 2. WebSub Support (Web Push)
     - Added flWebsubEnabled configuration flag
     - Added urlWebsubHub configuration for WebSub hub URL
     - Added pingWebsubHub function to notify WebSub hubs when feeds are
       updated
     - Modified feed update logic to ping both RSS Cloud and WebSub hubs
     - Added WebSub headers to feed responses when enabled

 3. Configuration Updates
     - Updated RSS documentation URL from HTTP to HTTPS
     - Changed feed link construction to use config.urlServerForClient instead
       of hardcoded domain
     - Added new configuration parameters for WebSub support

 4. Minor Behavioral Changes
     - Feed URL construction now uses config.urlServerForClient consistently
     - Feed responses now include WebSub headers when enabled

 Impact on Go Implementation

 The Go implementation should be largely unaffected by these changes since:
 1. The Go port doesn't currently implement the /readhttpfile endpoint
 2. The Go port doesn't implement WebSub support (it only has RSS Cloud
    support)
 3. The core API endpoints (newpost, getitembyguid, etc.) remain unchanged
 4. The changes are mostly related to feed publishing and discovery mechanisms

 The most significant change is the addition of WebSub support, which is a new
 feature that would require additional implementation in the Go version if
 desired, but doesn't break existing functionality.

 The Go port remains compatible with the upstream changes and maintains the
 same core API contract that clients depend on.
