package names

// Things you'd run into on a New York block. Lowercase, hyphenated.
var nouns = []string{
	// critters
	"pigeon", "pizza-rat", "bodega-cat", "raccoon", "squirrel", "seagull",
	"hawk", "rat",
	// food
	"bagel", "knish", "slice", "pretzel", "hot-dog", "egg-cream", "lox",
	"schmear", "cannoli", "dumpling", "empanada", "chopped-cheese", "halal-cart",
	"cheesecake", "bialy", "babka", "rugelach", "pastrami",
	"seltzer", "hero", "dollar-slice", "papaya-dog",
	// street
	"stoop", "hydrant", "yellow-cab", "metrocard", "pothole", "manhole",
	"scaffold", "fire-escape", "water-tower", "brownstone", "traffic-cone",
	"bodega", "deli", "ferry", "tugboat", "citi-bike", "subway", "local",
	"sidewalk-shed", "steam-pipe", "newsstand", "stoop-sale",
	"walk-up", "rooftop", "bike-lane",
	// transit
	"turnstile", "straphanger", "dollar-van", "car-service",
	"bus-lane", "showtime", "rush-hour",
	// people
	"cabbie", "doorman", "super", "busker", "hustler", "bike-messenger",
	"landlord", "tourist", "commuter", "rat-czar", "dog-walker",
	// public office
	"mayor", "deputy-mayor", "comptroller", "public-advocate",
	"borough-president", "council-member", "governor", "night-mayor",
	"traffic-agent",
	// gen z and gen alpha slang
	"rizzler", "unc", "goat", "sigma", "npc", "main-character", "aura-farmer",
	"yapper", "bestie", "pookie", "delulu",
	// talk
	"schlep", "mensch", "kvetch",
	// chess, Washington Square style
	"rook", "knight", "bishop", "pawn", "blitz", "gambit", "castle",
	"en-passant", "zugzwang",
	// gta iii and vice city characters
	"vercetti", "lance-vance", "kent-paul", "phil-cassidy",
}

// New York places: NYC boroughs and neighborhoods first, then popular
// counties and towns from Wikipedia's lists of New York counties and towns,
// then a few famous villages, hamlets, areas and cities that are on neither
// list, then Liberty City, the New York of GTA III.
// How they were chosen: docs/guest-names.md.
var places = []string{
	// boroughs and counties
	"bronx", "brooklyn", "queens", "staten-island", "manhattan", "richmond",
	// manhattan
	"harlem", "inwood", "soho", "noho", "tribeca", "chelsea", "nolita", "fidi",
	"midtown", "ues", "uws", "kips-bay", "murray-hill", "hells-kitchen",
	"chinatown", "east-village", "the-village",
	// brooklyn
	"flatbush", "bushwick", "dumbo", "gowanus", "red-hook", "bed-stuy",
	"park-slope", "bay-ridge", "canarsie", "greenpoint", "williamsburg",
	"coney-island", "bensonhurst", "sunset-park", "crown-heights",
	// queens
	"astoria", "sunnyside", "woodside", "flushing", "jamaica", "rockaway",
	"corona", "elmhurst", "ridgewood", "forest-hills", "kew-gardens",
	"jackson-heights",
	// bronx
	"mott-haven", "fordham", "riverdale", "pelham", "throgs-neck",
	"hunts-point",
	// staten island
	"st-george", "tottenville",
	// counties
	"westchester", "nassau", "suffolk", "albany",
	"rockland", "saratoga", "niagara",
	// towns: long island
	"hempstead", "oyster-bay", "huntington", "babylon",
	"southampton", "east-hampton", "shelter-island",
	// towns: westchester
	"rye", "scarsdale", "greenburgh", "bedford", "ossining",
	"yorktown",
	// towns: hudson valley, catskills, finger lakes
	"poughkeepsie", "hyde-park", "rhinebeck", "fishkill", "woodstock",
	"new-paltz", "kingston",
	"ithaca",
	// famous, but not counties or towns
	"montauk", "yonkers", "tarrytown", "sleepy-hollow", "beacon",
	"cooperstown", "lake-placid", "the-hamptons", "fire-island",
	// cities
	"buffalo", "rochester", "syracuse",
	// liberty city, gta iii's new york
	"liberty-city", "staunton", "shoreside-vale", "hepburn-heights",
	"saint-marks", "bedford-point", "belleville-park", "fort-staunton",
	"wichita-gardens", "callahan-point", "torrington",
}

// Pairs that read badly together, mostly as a stereotype about who lives
// somewhere. Checked as "noun-place"; every entry must be a real pair
// (TestBlockedPairsExist).
var blocked = map[string]bool{
	"hustler-harlem": true, "hustler-bed-stuy": true, "hustler-mott-haven": true,
	"hustler-hunts-point": true, "hustler-bronx": true, "hustler-canarsie": true,
	"hustler-jamaica": true,
	"rat-chinatown":   true, "pizza-rat-chinatown": true, "pigeon-chinatown": true,
	"raccoon-chinatown": true, "rat-flushing": true, "pizza-rat-flushing": true,
	"landlord-harlem": true,
	"npc-chinatown":   true, "npc-flushing": true,
	"rat-sunset-park": true, "pigeon-sunset-park": true, "raccoon-sunset-park": true,
	"npc-sunset-park": true, "rat-elmhurst": true, "pizza-rat-elmhurst": true,
	"pigeon-elmhurst": true, "raccoon-elmhurst": true, "npc-elmhurst": true,
	"vercetti-bensonhurst": true, "vercetti-harlem": true, "vercetti-bed-stuy": true,
	"vercetti-mott-haven": true, "vercetti-hunts-point": true, "vercetti-bronx": true,
	"vercetti-canarsie": true, "vercetti-jamaica": true,
	"lance-vance-harlem": true, "lance-vance-bronx": true, "lance-vance-canarsie": true,
	"lance-vance-jamaica": true,
}
