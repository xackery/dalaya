package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

var zoneName = "soldungb"
var isSkipFetch = false
var isAllZones = false

type item struct {
	ItemID      int     `db:"item_id"`
	ItemName    string  `db:"item_name"`
	ItemSlots   int     `db:"item_slots"`
	ItemHP      int     `db:"item_hp"`
	ItemMana    int     `db:"item_mana"`
	ItemClasses int     `db:"item_classes"`
	NpcID       int     `db:"npc_id"`
	NpcName     string  `db:"npc_name"`
	NpcHP       int     `db:"npc_hp"`
	NpcLevel    int     `db:"npc_level"`
	DropChance  float32 `db:"chance"`
	Classes     string
	Awakened    *itemAwakened
}

type itemAwakened struct {
	Name     string `db:"name"`
	HealAmt  int    `db:"healamt"`
	SpellAmt int    `db:"spelldmg"`
	HP       int    `db:"hp"`
	Mana     int    `db:"mana"`
}

func (e *itemAwakened) String() string {
	return fmt.Sprintf("%s: %d hp, %d mana, %d heal, %d spell", strings.TrimSuffix(e.Name, " (Awakened)"), e.HP, e.Mana, e.HealAmt, e.SpellAmt)
}

type byHealAmt []*item

func (a byHealAmt) Len() int           { return len(a) }
func (a byHealAmt) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byHealAmt) Less(i, j int) bool { return a[i].Awakened.HealAmt > a[j].Awakened.HealAmt }

func main() {
	if isAllZones {
		for _, zone := range allZones {
			fi, err := os.Stat(fmt.Sprintf("../../content/zones/%s.md", zone))
			if err == nil && fi.Size() > 0 {
				continue
			}
			err = zoneDump(zone)
			if err != nil {
				fmt.Println("Failed:", err)
				os.Exit(1)
			}
		}
		return
	}
	err := zoneDump(zoneName)
	if err != nil {
		fmt.Println("Failed:", err)
		os.Exit(1)
	}

}

func zoneDump(zoneID string) error {
	fmt.Printf("Generating zone %s\n", zoneID)
	start := time.Now()
	var err error
	var db *sqlx.DB
	db, err = sqlx.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true", "peq", "peqpass", "127.0.0.1", "3306", "peq"))
	if err != nil {
		return fmt.Errorf("sql.Open: %w", err)
	}

	zoneShort, zoneLong, err := zone(context.Background(), db, zoneID)
	if err != nil {
		return fmt.Errorf("zone: %w", err)
	}

	fmt.Printf("Parsed NPCs in %0.2f seconds\n", time.Since(start).Seconds())
	start = time.Now()

	items, err := itemsByZoneShortName(context.Background(), db, zoneID)
	if err != nil {
		return fmt.Errorf("items: %w", err)
	}
	finals := []*item{}

	fmt.Printf("Parsed items in %0.2f seconds\n", time.Since(start).Seconds())
	/*
		lastNpc := ""
		 for _, item := range items {
			if item.ItemSlots == 0 {
				continue
			}
			if lastNpc == item.NpcName {
				continue
			}
			awakened, err := awakened(context.Background(), db, item.ItemID)
			if err != nil {
				//return fmt.Errorf("awakened: %w", err)
				continue
			}
			lastNpc = item.NpcName

			item.Awakened = awakened
			if awakened.HealAmt < 5 && awakened.SpellAmt < 5 {
				continue
			}

			finals = append(finals, item)

		} */

	sort.Sort(byHealAmt(finals))

	lastNPC := ""
	lastItem := ""
	for _, item := range finals {
		if lastNPC == item.NpcName && lastItem == item.ItemName {
			continue
		}
		lastNPC = item.NpcName
		lastItem = item.ItemName

	}

	allItems := []string{}
	allNPCs := []string{}

	zoneBuf := &bytes.Buffer{}
	fmt.Fprintf(zoneBuf, `---
title: %s
weight: 5
description: %s
bookToC: true
bookHidden: true
---
`, zoneLong, zoneLong)

	lastNPCName := ""
	lastItemIDs := []int{}
	knownNPCs := []string{}
	for _, item := range items {
		if item.DropChance < 0.1 {
			continue
		}
		if item.ItemSlots == 0 {
			continue
		}
		if item.NpcName != lastNPCName {
			isKnown := false
			for _, npc := range knownNPCs {
				if npc == item.NpcName {
					isKnown = true
					break
				}
			}
			if isKnown {
				continue
			}
			lastNPCName = item.NpcName
			lastItemIDs = []int{}
			fmt.Fprintf(zoneBuf, `## %s (%d)`+"\n", CleanName(item.NpcName), item.NpcLevel)
			fmt.Fprintf(zoneBuf, `- {{<npc id="%d" name="%s">}} (%d) has %d Hitpoints`+"\n", item.NpcID, CleanName(item.NpcName), item.NpcLevel, item.NpcHP)

			knownNPCs = append(knownNPCs, item.NpcName)
			isDuplicate := false
			for _, id := range allNPCs {
				if id == fmt.Sprintf("%d", item.NpcID) {
					isDuplicate = true
					break
				}
			}
			if !isDuplicate {
				allNPCs = append(allNPCs, fmt.Sprintf("%d", item.NpcID))
			}
		}
		isDuplicate := false
		for _, id := range lastItemIDs {
			if id == item.ItemID {
				isDuplicate = true
				break
			}
		}
		if isDuplicate {
			continue
		}

		isDuplicate = false
		for _, id := range allItems {
			if id == fmt.Sprintf("%d", item.ItemID) {
				isDuplicate = true
				break
			}
		}
		if !isDuplicate {
			allItems = append(allItems, fmt.Sprintf("%d", item.ItemID))
		}
		if item.ItemHP > 0 || item.ItemMana > 0 {
			fmt.Fprintf(zoneBuf, `- %0.1f%% {{<item id="%d" name="%s">}} with`, item.DropChance, item.ItemID, item.ItemName)
			if item.ItemHP > 0 {
				fmt.Fprintf(zoneBuf, " %d hp", item.ItemHP)
			}
			if item.ItemMana > 0 {
				fmt.Fprintf(zoneBuf, " %d mana", item.ItemMana)
			}
			fmt.Fprintf(zoneBuf, " for %s", item.Classes)
		} else if item.Classes != "" {
			fmt.Fprintf(zoneBuf, `- %0.1f%% {{<item id="%d" name="%s">}} for %s`, item.DropChance, item.ItemID, item.ItemName, item.Classes)
		} else {
			fmt.Fprintf(zoneBuf, `- %0.1f%% {{<item id="%d" name="%s">}}`, item.DropChance, item.ItemID, item.ItemName)
		}
		lastItemIDs = append(lastItemIDs, item.ItemID)
		fmt.Fprintf(zoneBuf, "\n")
	}

	sort.Strings(allItems)
	sort.Strings(allNPCs)

	if !isSkipFetch {
		npcIDs := []int{}
		for _, id := range allNPCs {
			npcID, err := strconv.Atoi(id)
			if err != nil {
				return fmt.Errorf("convert npc id: %w", err)
			}
			npcIDs = append(npcIDs, npcID)

		}

		err = npcDump(npcIDs, fmt.Sprintf("../../assets/zones/%s-npcs.yaml", zoneShort))
		if err != nil {
			return fmt.Errorf("npc dump: %w", err)
		}

		itemIDs := []int{}
		for _, id := range allItems {
			itemID, err := strconv.Atoi(id)
			if err != nil {
				return fmt.Errorf("convert item id: %w", err)
			}
			itemIDs = append(itemIDs, itemID)
		}
		sort.Ints(itemIDs)

		err = itemDump(itemIDs, fmt.Sprintf("../../assets/zones/%s-items.yaml", zoneShort))
		if err != nil {
			return fmt.Errorf("item dump: %w", err)
		}

	}
	err = os.WriteFile(fmt.Sprintf("../../content/zones/%s.md", zoneShort), zoneBuf.Bytes(), 0644)
	if err != nil {
		return fmt.Errorf("write zone: %w", err)
	}

	return nil
}

func itemDump(itemIDs []int, path string) error {
	fmt.Printf("Generating items\n")
	start := time.Now()
	var err error

	sort.Ints(itemIDs)

	var itemBuf = &bytes.Buffer{}

	for _, id := range itemIDs {

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

	fmt.Printf("Fetched %d items in %0.2f seconds\n", len(itemIDs), time.Since(start).Seconds())

	err = os.WriteFile(path, itemBuf.Bytes(), 0644)
	if err != nil {
		return fmt.Errorf("write items: %w", err)
	}

	return nil
}

func npcDump(npcIDs []int, path string) error {
	fmt.Printf("Generating NPCs\n")
	start := time.Now()
	var err error

	sort.Ints(npcIDs)

	var npcBuf = &bytes.Buffer{}

	for _, id := range npcIDs {
		fmt.Printf("%d... ", id)
		resp, err := http.Get(fmt.Sprintf("http://localhost:8080/npc/peek?id=%d", id))
		if err != nil {
			return fmt.Errorf("peek npcs: %w", err)
		}
		if resp.StatusCode != 200 {
			return fmt.Errorf("peek npcs: %s", resp.Status)
		}
		_, err = npcBuf.ReadFrom(resp.Body)
		if err != nil {
			return fmt.Errorf("read npc body: %w", err)
		}
	}

	fmt.Printf("\n")

	fmt.Printf("Fetched %d NPCs in %0.2f seconds\n", len(npcIDs), time.Since(start).Seconds())

	err = os.WriteFile(path, npcBuf.Bytes(), 0644)
	if err != nil {
		return fmt.Errorf("write npcs: %w", err)
	}

	return nil
}

func itemsByZoneShortName(ctx context.Context, db *sqlx.DB, zone string) ([]*item, error) {
	results, err := db.NamedQueryContext(ctx, `select i.id item_id, i.name item_name, i.slots item_slots, i.hp item_hp, i.mana item_mana, i.classes item_classes, n.id npc_id, n.name npc_name, n.hp npc_hp, n.level npc_level, lde.chance FROM items i
	inner join lootdrop_entries lde on lde.item_id = i.id
	inner join loottable_entries lte on lte.lootdrop_id = lde.lootdrop_id
	inner join npc_types n on n.loottable_id = lte.loottable_id
	inner join spawnentry se on se.npcID = n.id
	inner join spawn2 s2 on s2.spawngroupID = se.spawngroupID
	where s2.zone = :zone ORDER BY n.id`, map[string]interface{}{"zone": zone})
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}

	var items []*item
	for results.Next() {
		entry := &item{}
		err = results.StructScan(&entry)
		if err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		entry.Classes = ClassesFromMask(int32(entry.ItemClasses))
		if entry.ItemID >= 10048 && entry.ItemID <= 10052 {
			continue
		}
		items = append(items, entry)
	}

	return items, nil
}

func zone(ctx context.Context, db *sqlx.DB, zoneShort string) (string, string, error) {
	results, err := db.NamedQueryContext(ctx, `select short_name, long_name from zone where short_name = :zoneID`, map[string]interface{}{"zoneID": zoneShort})
	if err != nil {
		return "", "", fmt.Errorf("query: %w", err)
	}

	var shortName, longName string
	for results.Next() {
		err = results.Scan(&shortName, &longName)
		if err != nil {
			return "", "", fmt.Errorf("scan: %w", err)
		}
	}

	return shortName, longName, nil
}

// ClassesFromMask returns a string of classes from a bitmask
func ClassesFromMask(in int32) string {
	out := ""

	if in == 65535 {
		return "ALL"
	}
	if in&1 != 0 {
		out += "WAR "
	}
	if in&2 != 0 {
		out += "CLR "
	}
	if in&4 != 0 {
		out += "PAL "
	}
	if in&8 != 0 {
		out += "RNG "
	}
	if in&16 != 0 {
		out += "SHD "
	}
	if in&32 != 0 {
		out += "DRU "
	}
	if in&64 != 0 {
		out += "MNK "
	}
	if in&128 != 0 {
		out += "BRD "
	}
	if in&256 != 0 {
		out += "ROG "
	}
	if in&512 != 0 {
		out += "SHM "
	}
	if in&1024 != 0 {
		out += "NEC "
	}
	if in&2048 != 0 {
		out += "WIZ "
	}
	if in&4096 != 0 {
		out += "MAG "
	}
	if in&8192 != 0 {
		out += "ENC "
	}
	if in&16384 != 0 {
		out += "BST "
	}

	out = strings.TrimSuffix(out, " ")
	return out
}

func CleanName(in string) string {
	out := in
	out = strings.ReplaceAll(out, "_", " ")
	out = strings.ReplaceAll(out, "-", "`")
	out = strings.ReplaceAll(out, "#", "")
	out = strings.ReplaceAll(out, "!", "")
	out = strings.ReplaceAll(out, "~", "")
	return out
}

var allZones = []string{
	"qeynos",
	"qeynos2",
	"qrg",
	"qeytoqrg",
	"highkeep",
	"freportn",
	"freportw",
	"freporte",
	"runnyeye",
	"qey2hh1",
	"northkarana",
	"southkarana",
	"eastkarana",
	"beholder",
	"blackburrow",
	"paw",
	"rivervale",
	"kithicor",
	"commons",
	"ecommons",
	"erudnint",
	"erudnext",
	"nektulos",
	"cshome",
	"lavastorm",
	"halas",
	"everfrost",
	"soldunga",
	"soldungb",
	"misty",
	"nro",
	"southro",
	"befallen",
	"oasis",
	"tox",
	"hole",
	"neriaka",
	"neriakb",
	"neriakc",
	"neriakd",
	"najena",
	"qcat",
	"innothule",
	"feerrott",
	"cazicthule",
	"oggok",
	"rathemtn",
	"lakerathe",
	"grobb",
	"aviak",
	"gfaydark",
	"akanon",
	"steamfont",
	"lfaydark",
	"crushbone",
	"mistmoore",
	"kaladima",
	"felwithea",
	"felwitheb",
	"unrest",
	"kedge",
	"guktop",
	"gukbottom",
	"kaladimb",
	"butcher",
	"oot",
	"cauldron",
	"airplane",
	"fearplane",
	"permafrost",
	"kerraridge",
	"paineel",
	"hateplane",
	"arena",
	"soltemple",
	"erudsxing",
	"stonebrunt",
	"warrens",
	"erudsxing2",
	"bazaar",
	"jaggedpine",
	"nedaria",
	"hateplaneb",
	"shadowrest",
	"tutoriala",
	"tutorialb",
	"poknowledge",
	"soldungc",
	"guildlobby",
	"barter",
	"takishruins",
	"freeporteast",
	"freeportwest",
	"freeportsewers",
	"northro",
	"southro",
	"highpasshold",
	"commonlands",
	"oceanoftears",
	"Keep",
	"innothuleb",
	"toxxulia",
	"mistythicket",
	"steamfontmts",
	"dragonscalea",
	"crafthalls",
	"weddingchapel",
	"weddingchapeldark",
	"dragoncrypt",
	"arttest",
	"fhalls",
	"apprentice",
	/* "abysmal",
	"acrylia",
	"airplane",
	"akanon",
	"akheva",
	"alkabormare",
	"anguish",
	"apprentice",
	"arcstone",
	"arelis",
	"arena",
	"arena2",
	"argath",
	"arthicrex",
	"arttest",
	"ashengate",
	"atiiki",
	"aviak",
	"barindu",
	"barren",
	"barter",
	"bazaar",
	"bazaar",
	"beastdomain",
	"befallen",
	"befallenb",
	"beholder",
	"bertoxtemple",
	"blackburrow",
	"blacksail",
	"bloodfields",
	"bloodmoon",
	"bothunder",
	"breedinggrounds",
	"brellsarena",
	"brellsrest",
	"brellstemple",
	"broodlands",
	"buriedsea",
	"burningwood",
	"butcher",
	"cabeast",
	"cabwest",
	"cauldron",
	"causeway",
	"cazicthule",
	"chambersa",
	"chambersa",
	"chambersa",
	"chambersb",
	"chambersb",
	"chambersb",
	"chambersc",
	"chambersc",
	"chambersc",
	"chambersd",
	"chambersd",
	"chambersd",
	"chamberse",
	"chamberse",
	"chamberse",
	"chambersf",
	"chambersf",
	"chambersf",
	"chapterhouse",
	"charasis",
	"chardok",
	"chardokb",
	"citymist",
	"citymist",
	"cityofbronze",
	"clz",
	"cobaltscar",
	"codecay",
	"commonlands",
	"commons",
	"convorteum",
	"coolingchamber",
	"corathus",
	"corathusa",
	"corathusa",
	"corathusa",
	"corathusb",
	"corathusb",
	"corathusb",
	"corathusb",
	"crescent",
	"crushbone",
	"cryptofshade",
	"crystal",
	"crystallos",
	"crystalshard",
	"cshome",
	"dalnir",
	"dawnshroud",
	"deadbone",
	"delvea",
	"delvea",
	"delvea",
	"delvea",
	"delvea",
	"delvea",
	"delvea",
	"delvea",
	"delvea",
	"delvea",
	"delveb",
	"delveb",
	"delveb",
	"delveb",
	"delveb",
	"delveb",
	"delveb",
	"devastation",
	"devastationa",
	"direwind",
	"discord",
	"discordtower",
	"drachnidhive",
	"drachnidhive",
	"drachnidhive",
	"drachnidhive",
	"drachnidhive",
	"drachnidhive",
	"drachnidhivea",
	"drachnidhivea",
	"drachnidhivea",
	"drachnidhivea",
	"drachnidhiveb",
	"drachnidhiveb",
	"drachnidhivec",
	"drachnidhivec",
	"dragoncrypt",
	"dragonscale",
	"dragonscaleb",
	"dranik",
	"dranikcatacombsa",
	"dranikcatacombsb",
	"dranikcatacombsc",
	"dranikhollowsa",
	"dranikhollowsb",
	"dranikhollowsc",
	"draniksewersa",
	"draniksewersb",
	"draniksewersc",
	"draniksscar",
	"dreadlands",
	"dreadspire",
	"dreadspire",
	"droga",
	"droga",
	"dulak",
	"eastkarana",
	"eastkorlach",
	"eastkorlach",
	"eastkorlacha",
	"eastkorlacha",
	"eastkorlacha",
	"eastkorlacha",
	"eastsepulcher",
	"eastwastes",
	"eastwastesshard",
	"echo",
	"ecommons",
	"elddar",
	"elddara",
	"emeraldjungle",
	"erudnext",
	"erudnint",
	"erudsxing",
	"erudsxing2",
	"everfrost",
	"eviltree",
	"fallen",
	"fearplane",
	"feerrott",
	"feerrott2",
	"felwithea",
	"felwitheb",
	"ferubi",
	"fhalls",
	"fieldofbone",
	"firiona",
	"foundation",
	"freeportacademy",
	"freeportarena",
	"freeportcityhall",
	"freeporteast",
	"freeporthall",
	"freeportmilitia",
	"freeportsewers",
	"freeporttemple",
	"freeporttheater",
	"freeportwest",
	"freporte",
	"freportn",
	"freportw",
	"frontiermtns",
	"frostcrypt",
	"frozenshadow",
	"fungalforest",
	"fungusgrove",
	"gfaydark",
	"greatdivide",
	"grelleth",
	"griegsend",
	"griegsend",
	"grimling",
	"grobb",
	"growthplane",
	"guardian",
	"guildhall",
	"guildlobby",
	"guka",
	"gukb",
	"gukbottom",
	"gukc",
	"gukc",
	"gukd",
	"guke",
	"gukf",
	"gukg",
	"gukg",
	"gukh",
	"guktop",
	"gunthak",
	"gyrospireb",
	"gyrospirez",
	"halas",
	"harbingers",
	"hateplane",
	"hateplaneb",
	"hatesfury",
	"highkeep",
	"highpass",
	"highpasshold",
	"highpasskeep",
	"hillsofshade",
	"hohonora",
	"hohonorb",
	"hole",
	"hollowshade",
	"housegarden",
	"iceclad",
	"icefall",
	"ikkinz",
	"illsalin",
	"illsalina",
	"illsalina",
	"illsalinb",
	"illsalinb",
	"illsalinb",
	"illsalinb",
	"illsalinb",
	"illsalinc",
	"illsalinc",
	"illsalinc",
	"illsalinc",
	"illsalinc",
	"inktuta",
	"innothule",
	"innothuleb",
	"jaggedpine",
	"jardelshook",
	"kael",
	"kaelshard",
	"kaesora",
	"kaladima",
	"kaladimb",
	"karnor",
	"katta",
	"kattacastrum",
	"kedge",
	"kerraridge",
	"kithforest",
	"kithicor",
	"kodtaz",
	"korascian",
	"kurn",
	"lakeofillomen",
	"lakerathe",
	"lavastorm",
	"lavastorm",
	"letalis",
	"lfaydark",
	"lichencreep",
	"load",
	"load2",
	"lopingplains",
	"maiden",
	"maidensgrave",
	"mansion",
	"mechanotus",
	"mesa",
	"mira",
	"miragulmare",
	"mirb",
	"mirb",
	"mirc",
	"mirc",
	"mird",
	"mire",
	"mirf",
	"mirg",
	"mirh",
	"miri",
	"mirj",
	"mischiefplane",
	"mistmoore",
	"misty",
	"mistythicket",
	"mmca",
	"mmcb",
	"mmcc",
	"mmcc",
	"mmcd",
	"mmce",
	"mmcf",
	"mmcf",
	"mmcg",
	"mmch",
	"mmci",
	"mmcj",
	"monkeyrock",
	"moors",
	"morellcastle",
	"mseru",
	"nadox",
	"najena",
	"natimbi",
	"necropolis",
	"nedaria",
	"neighborhood",
	"nektropos",
	"nektulos",
	"nektulos",
	"nektulosa",
	"neriaka",
	"neriakb",
	"neriakc",
	"neriakd",
	"netherbian",
	"nexus",
	"nightmareb",
	"northkarana",
	"northro",
	"nro",
	"nurga",
	"nurga",
	"oasis",
	"oceangreenhills",
	"oceangreenvillage",
	"oceanoftears",
	"oggok",
	"oldblackburrow",
	"oldbloodfield",
	"oldcommons",
	"olddranik",
	"oldfieldofbone",
	"oldhighpass",
	"oldkaesoraa",
	"oldkaesorab",
	"oldkithicor",
	"oldkurn",
	"oot",
	"overthere",
	"paineel",
	"paludal",
	"paw",
	"paw",
	"pellucid",
	"permafrost",
	"pillarsalra",
	"poair",
	"podisease",
	"poeartha",
	"poearthb",
	"pofire",
	"poinnovation",
	"pojustice",
	"poknowledge",
	"ponightmare",
	"postorms",
	"potactics",
	"potimea",
	"potimeb",
	"potorment",
	"potranquility",
	"povalor",
	"powar",
	"powater",
	"precipiceofwar",
	"provinggrounds",
	"qcat",
	"qey2hh1",
	"qeynos",
	"qeynos2",
	"qeytoqrg",
	"qinimi",
	"qrg",
	"qvic",
	"qvicb",
	"rage",
	"ragea",
	"rathechamber",
	"rathemtn",
	"redfeather",
	"relic",
	"resplendent",
	"riftseekers",
	"rivervale",
	"riwwi",
	"roost",
	"rubak",
	"ruja",
	"rujb",
	"rujc",
	"rujd",
	"rujd",
	"ruje",
	"rujf",
	"rujg",
	"rujg",
	"rujh",
	"ruji",
	"rujj",
	"rujj",
	"runnyeye",
	"sarithcity",
	"scarlet",
	"sebilis",
	"sepulcher",
	"shadeweaver",
	"shadowhaven",
	"shadowrest",
	"shadowspine",
	"shadowspine",
	"shardslanding",
	"sharvahl",
	"shiningcity",
	"shipmvm",
	"shipmvp",
	"shipmvu",
	"shippvu",
	"shipuvu",
	"shipworkshop",
	"silyssar",
	"sirens",
	"sirens",
	"skyfire",
	"skylance",
	"skyshrine",
	"sleeper",
	"sncrematory",
	"snlair",
	"snplant",
	"snpool",
	"soldunga",
	"soldungb",
	"soldungc",
	"solrotower",
	"soltemple",
	"solteris",
	"somnium",
	"southkarana",
	"southro",
	"sro",
	"sseru",
	"ssratemple",
	"steamfactory",
	"steamfont",
	"steamfontmts",
	"steppes",
	"stillmoona",
	"stillmoona",
	"stillmoona",
	"stillmoona",
	"stillmoona",
	"stillmoona",
	"stillmoona",
	"stillmoona",
	"stillmoona",
	"stillmoona",
	"stillmoonb",
	"stillmoonb",
	"stillmoonb",
	"stillmoonb",
	"stillmoonb",
	"stillmoonb",
	"stillmoonb",
	"stillmoonb",
	"stonebrunt",
	"stonehive",
	"stonesnake",
	"suncrest",
	"sunderock",
	"swampofnohope",
	"tacvi",
	"taka",
	"takb",
	"takc",
	"takd",
	"take",
	"take",
	"takf",
	"takg",
	"takh",
	"taki",
	"takishruins",
	"takishruinsa",
	"takj",
	"templeveeshan",
	"tenebrous",
	"thalassius",
	"theater",
	"theatera",
	"thedeep",
	"thegrey",
	"thenest",
	"thenest",
	"thenest",
	"thenest",
	"thenest",
	"thenest",
	"thenest",
	"thenest",
	"thenest",
	"thenest",
	"thenest",
	"thenest",
	"thenest",
	"thenest",
	"thenest",
	"thenest",
	"thevoida",
	"thevoidb",
	"thevoidc",
	"thevoidd",
	"thevoide",
	"thevoidf",
	"thevoidg",
	"thuledream",
	"thulehouse1",
	"thulehouse2",
	"thulelibrary",
	"thundercrest",
	"thundercrest",
	"thundercrest",
	"thundercrest",
	"thundercrest",
	"thundercrest",
	"thundercrest",
	"thundercrest",
	"thundercrest",
	"thundercrest",
	"thundercrest",
	"thundercrest",
	"thundercrest",
	"thundercrest",
	"thundercrest",
	"thundercrest",
	"thurgadina",
	"thurgadinb",
	"timorous",
	"tipt",
	"torgiran",
	"toskirakk",
	"tox",
	"toxxulia",
	"trakanon",
	"tutorial",
	"tutoriala",
	"tutorialb",
	"twilight",
	"txevu",
	"umbral",
	"underquarry",
	"unrest",
	"uqua",
	"valdeholm",
	"veeshan",
	"veksar",
	"velketor",
	"vergalid",
	"vexthal",
	"vxed",
	"vxed",
	"wakening",
	"wallofslaughter",
	"warrens",
	"warslikswood",
	"weddingchapel",
	"weddingchapeldark",
	"well",
	"westkorlach",
	"westkorlach",
	"westkorlach",
	"westkorlach",
	"westkorlacha",
	"westkorlacha",
	"westkorlacha",
	"westkorlacha",
	"westkorlachb",
	"westkorlachb",
	"westkorlachc",
	"westkorlachc",
	"westkorlachc",
	"westkorlachc",
	"westkorlachc",
	"westkorlachc",
	"westkorlachc",
	"westsepulcher",
	"westwastes",
	"windsong",
	"xorbb",
	"yxtta",
	"zhisza", */
}
