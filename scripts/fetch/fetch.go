package main

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"sort"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var context = "classes"

var spells = []int{}
var npcs = []int{}
var items = []int{
	10650,
	20490,
	20542,
	28034,
	5532,
	68299,
	8495,
	8496,
}

func main() {
	err := run()
	if err != nil {
		fmt.Println("Failed:", err)
		os.Exit(1)
	}
}

func run() error {
	var err error
	npcBuf := new(bytes.Buffer)
	itemBuf := new(bytes.Buffer)
	spellBuf := new(bytes.Buffer)

	sort.Ints(spells)
	sort.Ints(npcs)
	sort.Ints(items)

	fmt.Printf("Generating %s\n", context)
	start := time.Now()
	if len(npcs) > 0 {
		for _, id := range npcs {
			fmt.Printf("%d... ", id)
			resp, err := http.Get(fmt.Sprintf("http://localhost:8080/npc/peek?id=%d", id))
			if err != nil {
				return fmt.Errorf("peek npcs %d: %w", id, err)
			}
			if resp.StatusCode != 200 {
				return fmt.Errorf("peek npcs %d: %s", id, resp.Status)
			}
			_, err = npcBuf.ReadFrom(resp.Body)
			if err != nil {
				return fmt.Errorf("read npc body: %w", err)
			}
		}
		fmt.Printf("\n")
		err = os.WriteFile(fmt.Sprintf("assets/%s/npcs.toml", context), npcBuf.Bytes(), 0644)
		if err != nil {
			return fmt.Errorf("write npcs: %w", err)
		}

		fmt.Printf("Fetched %d NPCs in %0.2f seconds\n", len(npcs), time.Since(start).Seconds())
		start = time.Now()
	}
	if len(items) > 0 {
		for _, id := range items {

			fmt.Printf("%d... ", id)
			resp2, err := http.Get(fmt.Sprintf("http://localhost:8080/item/peek?id=%d", id))
			if err != nil {
				return fmt.Errorf("peek items: %w", err)
			}
			if resp2.StatusCode != 200 {
				return fmt.Errorf("peek items: %s", resp2.Status)
			}
			_, err = itemBuf.ReadFrom(resp2.Body)
			if err != nil {
				return fmt.Errorf("read item body: %w", err)
			}
		}

		fmt.Printf("\n")

		fmt.Printf("Fetched %d items in %0.2f seconds\n", len(items), time.Since(start).Seconds())

		err = os.WriteFile(fmt.Sprintf("assets/%s/items.toml", context), itemBuf.Bytes(), 0644)
		if err != nil {
			return fmt.Errorf("write items: %w", err)
		}
	}

	if len(spells) > 0 {
		for _, id := range spells {
			fmt.Printf("%d... ", id)
			resp, err := http.Get(fmt.Sprintf("http://localhost:8080/spell/peek?id=%d", id))
			if err != nil {
				return fmt.Errorf("peek spells: %w", err)
			}
			if resp.StatusCode != 200 {
				return fmt.Errorf("peek spells: %s", resp.Status)
			}
			_, err = spellBuf.ReadFrom(resp.Body)
			if err != nil {
				return fmt.Errorf("read spell body: %w", err)
			}
		}
		fmt.Printf("\n")

		err = os.WriteFile(fmt.Sprintf("assets/%s/spells.toml", context), spellBuf.Bytes(), 0644)
		if err != nil {
			return fmt.Errorf("write spell: %w", err)
		}
	}

	return nil
}
