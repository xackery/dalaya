package main

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestZoneDump(t *testing.T) {
	err := zoneDump("soldungb")
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
}

func TestAll(t *testing.T) {
	TestTradeskills(t)
}

func TestTradeskills(t *testing.T) {
	dumpData(t,
		"../../content/tradeskills.md",
		"../../assets/page/tradeskills-items.yaml",
		"../../assets/page/tradeskills-npcs.yaml",
	)
}
func TestClasses(t *testing.T) {
	dumpData(t,
		"../../content/classes.md",
		"../../assets/page/classes-items.yaml",
		"../../assets/page/classes-npcs.yaml",
	)
}

func dumpData(t *testing.T, srcPath string, itemPath string, npcPath string) {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	reItem := regexp.MustCompile(`item id="(\d+)"`)
	reNpc := regexp.MustCompile(`npc id="(\d+)"`)

	itemIDs := []int{}
	npcIDs := []int{}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		matches := reItem.FindAllStringSubmatch(line, -1)
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}
			itemID, err := strconv.Atoi(match[1])
			if err != nil {
				t.Fatalf("Failed: %v", err)
			}
			itemIDs = append(itemIDs, itemID)
		}

		matches = reNpc.FindAllStringSubmatch(line, -1)
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}
			npcID, err := strconv.Atoi(match[1])
			if err != nil {
				t.Fatalf("Failed: %v", err)
			}
			npcIDs = append(npcIDs, npcID)
		}
	}

	err = itemDump(itemIDs, itemPath)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}

	err = npcDump(npcIDs, npcPath)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}

}
