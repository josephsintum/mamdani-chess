package rules

import "testing"

func TestSAN(t *testing.T) {
	cases := []struct {
		fen     string
		mamdani Square
		uci     string
		want    string
	}{
		{startFEN, A5, "e2e4", "e4"},
		{startFEN, A5, "g1f3", "Nf3"},
		{startFEN, A5, "a5b5", "Mb5"},
		{"r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1", NoSquare, "e1g1", "O-O"},
		{"r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1", NoSquare, "e1c1", "O-O-O"},
		{"4k3/P7/8/8/8/8/8/4K3 w - - 0 1", NoSquare, "a7a8q", "a8=Q+"},
		{"4k3/8/8/3p4/4P3/8/8/4K3 w - - 0 1", NoSquare, "e4d5", "exd5"},
		{"4k3/8/8/8/8/8/8/R4RK1 w - - 0 1", NoSquare, "a1d1", "Rad1"},
		{"4k3/8/8/8/8/R7/8/R3K3 w - - 0 1", NoSquare, "a1a2", "R1a2"},
		{"4k3/8/8/8/8/8/8/4K2Q w - - 0 1", NoSquare, "h1h5", "Qh5+"},
		{"rnbqkbnr/pppp1ppp/8/4p3/6P1/5P2/PPPPP2P/RNBQKBNR b KQkq g3 0 2", NoSquare, "d8h4", "Qh4#"},
	}
	for _, c := range cases {
		p := setup(t, c.fen, c.mamdani)
		if got := p.SAN(mv(t, c.uci)); got != c.want {
			t.Errorf("%s %s: got %q, want %q", c.fen, c.uci, got, c.want)
		}
	}
}
