package names

// Things you'd run into on a New York block. Lowercase, hyphenated.
var nouns = []string{
	// critters
	"pigeon", "pizza-rat", "bodega-cat", "raccoon", "squirrel", "seagull",
	"coyote", "hawk", "rat",
	// food
	"bagel", "knish", "slice", "pretzel", "hot-dog", "egg-cream", "lox",
	"schmear", "cannoli", "dumpling", "empanada", "chopped-cheese", "halal-cart",
	"cheesecake", "black-white",
	// street
	"stoop", "hydrant", "yellow-cab", "metrocard", "pothole", "manhole",
	"scaffold", "fire-escape", "water-tower", "brownstone", "traffic-cone",
	"bodega", "deli", "ferry", "tugboat", "citi-bike", "subway", "local", "express",
	// people
	"cabbie", "doorman", "super", "busker", "hustler", "bike-messenger",
	// chess, Washington Square style
	"rook", "knight", "bishop", "pawn", "blitz", "gambit",
}

// NYC neighborhoods and boroughs, plus a few county names (Kings, Queens).
var places = []string{
	// boroughs and counties
	"bronx", "brooklyn", "queens", "kings", "staten", "manhattan", "richmond",
	// manhattan
	"harlem", "inwood", "soho", "noho", "tribeca", "chelsea", "nolita", "fidi",
	"midtown", "les", "ues", "uws", "kips-bay", "murray-hill", "hells-kitchen",
	"two-bridges", "chinatown", "east-village", "the-village",
	// brooklyn
	"flatbush", "bushwick", "dumbo", "gowanus", "red-hook", "bed-stuy",
	"park-slope", "bay-ridge", "canarsie", "greenpoint", "williamsburg",
	"coney-island", "bensonhurst", "sunset-park", "crown-heights",
	// queens
	"astoria", "sunnyside", "woodside", "flushing", "jamaica", "rockaway",
	"corona", "elmhurst", "ridgewood", "forest-hills", "kew-gardens", "lic",
	"jackson-heights",
	// bronx
	"mott-haven", "fordham", "riverdale", "pelham", "throgs-neck",
	"hunts-point", "kingsbridge",
	// staten island
	"st-george", "tottenville",
}

// Pairs that read badly together. Checked as "noun-place".
var blocked = map[string]bool{}
