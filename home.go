package main

import (
    "encoding/json"
    "log"
    "strconv"
    "time"

    "github.com/docopt/docopt-go"
    "github.com/nbd-wtf/go-nostr"
)

func home(opts docopt.Opts, inboxMode bool) {

    initNostr()

    verbose, _ := opts.Bool("--verbose")
    jsonformat, _ := opts.Bool("--json")
    noreplies, _ := opts.Bool("--noreplies")
    onlyreplies, _ := opts.Bool("--onlyreplies")
    onlymentions, _ := opts.Bool("--onlymentions")
    kinds, kindserr := optSlice(opts, "--kinds")
    if kindserr != nil {
        return
    }
    var intkinds []int
    for _, kind := range kinds {
        if i, e := strconv.Atoi(kind); e == nil {
            intkinds = append(intkinds, i)
        }
    }
    since, _ := opts.Int("--since")
    until, _ := opts.Int("--until")
    limit, _ := opts.Int("--limit")
    references, _ := optSlice(opts, "--reference")

    // Mapping for alias to pubkeys used for output
    var keys []string
    nameMap := map[string]string{}
    for _, follow := range config.Following {
        keys = append(keys, follow.Key)
        if follow.Name != "" {
            nameMap[follow.Key] = follow.Name
        }
    }

    // Get our pubkey
    pubkey := getPubKey(config.PrivateKey)

    // Prepare filter to subscribe to based on options
    tags := make(map[string][]string)
    filters := nostr.Filters{{Limit: limit, Tags: nostr.TagMap{}}}
    // - override kinds for inbox to encrypted messages
    if inboxMode {
        intkinds = make([]int, 0)
        intkinds = append(intkinds, nostr.KindEncryptedDirectMessage)
    }
    filters[0].Kinds = intkinds
    // - Set publickey tag restrictions depending on mode or flags
    if inboxMode || onlymentions {
        // Filter by p tag to me
        tags["p"] = []string{pubkey}
    }
    // - Set event tag restrictions if reference set
    if len(references) > 0 {
        tags["e"] = []string{}
        for _, ref := range references {
            tags["e"] = append(tags["e"], ref)
        }
    }
    // - If no tags, filter by followers
    if len(tags) == 0 {
        // Filter to just those I follow
        filters[0].Authors = keys
    }
    // - Date range
    if since > 0 {
        sinceTime := time.Unix(int64(since), 0)
        filters[0].Since = &sinceTime
    }
    if until > 0 {
        untilTime := time.Unix(int64(until), 0)
        filters[0].Until = &untilTime
    }
    // - Assign assembled tags to filter
    filters[0].Tags = tags


    // Get from the relays
	_, all := pool.Sub(filters)
	for event := range nostr.Unique(all) {
		// Do we have a nick for the author of this message?
		nick, ok := nameMap[event.PubKey]
		if !ok {
			nick = ""
		}

		// If we don't already have a nick for this user, and they are announcing their
		// new name, let's use it.
		if nick == "" {
			if event.Kind == nostr.KindSetMetadata {
				var metadata Metadata
				err := json.Unmarshal([]byte(event.Content), &metadata)
				if err != nil {
					log.Println("Failed to parse metadata.")
					continue
				}
				nick = metadata.Name
				nameMap[nick] = event.PubKey
			}
		}

        // Post-filters (currently no way to filter up front for this)
        // if only want events referencing another
        if (onlyreplies || noreplies) {
            var hasReferences bool = false
            for _, tag := range event.Tags {
                if tag[0] == "e" {
                    hasReferences = true
                    if noreplies {
                        continue
                    }
                }
            }
            if (onlyreplies && !hasReferences) {
                continue
            }
        }

        // Render
		printEvent(event, &nick, verbose, jsonformat)
	}
}
