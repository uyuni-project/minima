package util

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

func ProcessPropertiesFile(reader io.Reader) (entries []map[string]string, err error) {
	entries = make([]map[string]string, 0)
	currentEntry := make(map[string]string)
	var key, value string

	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			// Empty line means end of previous entry, create a new one
			if key != "" {
				currentEntry[key] = value
				key = ""
				value = ""
			}

			entries = append(entries, currentEntry)
			currentEntry = make(map[string]string)
		} else {
			if line[0] != ' ' {
				// Line with no space: parse the key and value
				if key != "" {
					currentEntry[key] = value
					key = ""
					value = ""
				}

				k, v, found := strings.Cut(line, ":")
				if !found {
					err = fmt.Errorf("Invalid line: '%s'", line)
					return
				}
				key = strings.Trim(k, " ")
				value = strings.Trim(v, " ")
			} else {
				// Line with space: append to current value
				value = strings.Trim(value+"\n"+strings.Trim(line, " "), "\n")
			}
		}
	}
	// Ensure we have the last key/value pair and entry
	if key != "" {
		currentEntry[key] = value
	}

	if len(currentEntry) > 0 {
		entries = append(entries, currentEntry)
	}
	return
}
