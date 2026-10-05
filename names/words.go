package names

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

// Things you'd run into on a New York block. Lowercase, hyphenated.
var nouns = []string{
	// critters
	"pigeon", "pizza-rat", "bodega-cat", "raccoon", "squirrel", "seagull",
	"hawk",
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

// Pairs that read badly together, mostly as a stereotype about who lives
// somewhere: with the place first, a person word reads as a label for the
// people who live there ("harlem-hustler"). Checked as "place-noun"; every
// entry must be a real pair (TestBlockedPairsExist). Why each is here:
// docs/guest-names.md.
var blocked = map[string]bool{
	// crime, "loud" and slur tropes about Black and Latino neighborhoods ("coon" in raccoon)
	"harlem-hustler": true, "bed-stuy-hustler": true, "crown-heights-hustler": true,
	"canarsie-hustler": true, "jamaica-hustler": true, "bronx-hustler": true,
	"mott-haven-hustler": true, "hunts-point-hustler": true, "corona-hustler": true,
	"jackson-heights-hustler": true,
	"harlem-yapper":           true, "bed-stuy-yapper": true, "crown-heights-yapper": true,
	"canarsie-yapper": true, "jamaica-yapper": true, "bronx-yapper": true,
	"mott-haven-yapper": true, "hunts-point-yapper": true, "corona-yapper": true,
	"jackson-heights-yapper": true,
	"harlem-raccoon":         true, "bed-stuy-raccoon": true, "crown-heights-raccoon": true,
	"canarsie-raccoon": true, "jamaica-raccoon": true, "bronx-raccoon": true,
	"mott-haven-raccoon": true, "hunts-point-raccoon": true, "corona-raccoon": true,
	"jackson-heights-raccoon": true,
	// "they eat rats, pigeons and dogs", "all look alike" and "loud" tropes about Chinese neighborhoods
	"chinatown-yapper": true, "flushing-yapper": true, "sunset-park-yapper": true,
	"elmhurst-yapper":     true,
	"chinatown-pizza-rat": true, "flushing-pizza-rat": true, "sunset-park-pizza-rat": true,
	"elmhurst-pizza-rat": true,
	"chinatown-rat-czar": true, "flushing-rat-czar": true, "sunset-park-rat-czar": true,
	"elmhurst-rat-czar": true,
	"chinatown-pigeon":  true, "flushing-pigeon": true, "sunset-park-pigeon": true,
	"elmhurst-pigeon":   true,
	"chinatown-raccoon": true, "flushing-raccoon": true, "sunset-park-raccoon": true,
	"elmhurst-raccoon":  true,
	"chinatown-hot-dog": true, "flushing-hot-dog": true, "sunset-park-hot-dog": true,
	"elmhurst-hot-dog":     true,
	"chinatown-papaya-dog": true, "flushing-papaya-dog": true, "sunset-park-papaya-dog": true,
	"elmhurst-papaya-dog":  true,
	"chinatown-dog-walker": true, "flushing-dog-walker": true, "sunset-park-dog-walker": true,
	"elmhurst-dog-walker": true,
	"chinatown-npc":       true, "flushing-npc": true, "sunset-park-npc": true, "elmhurst-npc": true,
	// slumlord and complainer tropes about Hasidic neighborhoods; gentrification
	"harlem-landlord": true, "bed-stuy-landlord": true, "williamsburg-landlord": true,
	"crown-heights-landlord": true,
	"williamsburg-kvetch":    true, "crown-heights-kvetch": true,
	// taxi-driver stereotype
	"jackson-heights-cabbie": true,
	// GTA's crime bosses next to real neighborhoods
	"bensonhurst-vercetti": true, "harlem-vercetti": true, "bed-stuy-vercetti": true,
	"mott-haven-vercetti": true, "hunts-point-vercetti": true, "bronx-vercetti": true,
	"canarsie-vercetti": true, "jamaica-vercetti": true,
	"harlem-lance-vance": true, "bronx-lance-vance": true, "canarsie-lance-vance": true,
	"jamaica-lance-vance": true,
}
