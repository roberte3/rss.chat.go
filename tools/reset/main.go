package main

import (
	"flag"
	"fmt"
	"os"
)

var (
	dbPath           = flag.String("db", "rss.chat.db", "path to main database")
	mediaDBPath      = flag.String("mediadb", "rss.chat.media.db", "path to media database")
	feedsDBPath      = flag.String("feedsdb", "rss.chat.feeds.db", "path to feeds database")
	settingsPath     = flag.String("settings", "settings.json", "path to settings file")
	blocklistPath    = flag.String("blocklist", "blocklist.json", "path to blocklist file")
	configPath       = flag.String("config", "config.json", "path to config file")
	feedsDir         = flag.String("feedsdir", "feeds", "path to feeds directory")
	tempMediaPath    = flag.String("tempmedia", "temp_media", "path to temp media directory")
	keepConfig       = flag.Bool("keep-config", false, "preserve config.json (don't delete)")
	keepFeeds        = flag.Bool("keep-feeds", true, "preserve feeds directory (don't delete)")
	force            = flag.Bool("force", false, "skip confirmation prompt")
	verbose          = flag.Bool("v", false, "verbose output")
)

func main() {
	flag.Parse()

	if !*force {
		fmt.Println("⚠️  This will delete:")
		fmt.Printf("  - %s\n", *dbPath)
		fmt.Printf("  - %s\n", *mediaDBPath)
		fmt.Printf("  - %s\n", *feedsDBPath)
		fmt.Printf("  - %s\n", *settingsPath)
		fmt.Printf("  - %s\n", *blocklistPath)
		if !*keepConfig {
			fmt.Printf("  - %s\n", *configPath)
		}
		if !*keepFeeds {
			fmt.Printf("  - %s/\n", *feedsDir)
		}
		fmt.Printf("  - %s/\n", *tempMediaPath)
		fmt.Print("\nAre you sure? Type 'yes' to confirm: ")

		var response string
		fmt.Scanln(&response)
		if response != "yes" {
			fmt.Println("Cancelled.")
			return
		}
	}

	deleted := 0
	failed := 0

	// Delete databases
	files := []string{*dbPath, *mediaDBPath, *feedsDBPath, *settingsPath, *blocklistPath}
	if !*keepConfig {
		files = append(files, *configPath)
	}

	for _, f := range files {
		if err := deleteFile(f); err != nil {
			fmt.Printf("❌ Failed to delete %s: %v\n", f, err)
			failed++
		} else {
			if *verbose || err == nil {
				fmt.Printf("✓ Deleted %s\n", f)
			}
			deleted++
		}
	}

	// Delete directories
	dirs := []string{*tempMediaPath}
	if !*keepFeeds {
		dirs = append(dirs, *feedsDir)
	}

	for _, d := range dirs {
		if err := deleteDir(d); err != nil {
			fmt.Printf("❌ Failed to delete %s/: %v\n", d, err)
			failed++
		} else {
			if *verbose || err == nil {
				fmt.Printf("✓ Deleted %s/\n", d)
			}
			deleted++
		}
	}

	fmt.Printf("\n✅ Reset complete: %d deleted", deleted)
	if failed > 0 {
		fmt.Printf(", %d failed", failed)
	}
	fmt.Println()

	if *keepConfig {
		fmt.Println("\n⚠️  config.json preserved (use -keep-config=false to delete)")
	}
	if *keepFeeds {
		fmt.Println("⚠️  feeds/ directory preserved (use -keep-feeds=false to delete)")
	}

	if failed > 0 {
		os.Exit(1)
	}
}

func deleteFile(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist, consider it success
		}
		return err
	}
	if fi.IsDir() {
		return fmt.Errorf("is a directory, not a file")
	}
	return os.Remove(path)
}

func deleteDir(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Directory doesn't exist, consider it success
		}
		return err
	}
	if !fi.IsDir() {
		return fmt.Errorf("is a file, not a directory")
	}
	return os.RemoveAll(path)
}
