package store

import "context"

// Honk adds one to the skeleton counter and returns the new value.
// Throwaway: removed by the game-server plan.
func (s *Store) Honk(ctx context.Context) (int64, error) {
	var n int64
	err := s.db.QueryRowContext(ctx,
		`UPDATE honks SET count = count + 1 WHERE id = 1 RETURNING count`).Scan(&n)
	return n, err
}

// Honks returns the current counter value.
func (s *Store) Honks(ctx context.Context) (int64, error) {
	var n int64
	err := s.db.QueryRowContext(ctx, `SELECT count FROM honks WHERE id = 1`).Scan(&n)
	return n, err
}
