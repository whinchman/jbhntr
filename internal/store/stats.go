// Package store provides PostgreSQL-backed persistence for jobhuntr.
package store

import (
	"context"
	"fmt"
	"time"

	"github.com/whinchman/jobhuntr/internal/models"
)

// UserJobStats holds aggregate counts of a user's jobs by status.
type UserJobStats struct {
	TotalFound        int
	TotalApproved     int
	TotalRejected     int
	TotalApplied      int
	TotalInterviewing int
	TotalWon          int
	TotalLost         int
}

// WeeklyJobCount holds the number of jobs discovered in a single calendar week.
type WeeklyJobCount struct {
	WeekStart time.Time
	Count     int
}

// GetUserJobStats returns aggregate job counts for the given user using a
// single conditional-aggregation query. The status column is used for
// approved/rejected counts (existing pipeline statuses), and the
// application_status column is used for applied/interviewing/won/lost counts
// (added by migration 011).
func (s *Store) GetUserJobStats(ctx context.Context, userID int64) (UserJobStats, error) {
	const q = `
SELECT
    COUNT(*)                                                                    AS total_found,
    COUNT(*) FILTER (WHERE status = $2)                                         AS total_approved,
    COUNT(*) FILTER (WHERE status = $3)                                         AS total_rejected,
    COUNT(*) FILTER (WHERE application_status = $4)                             AS total_applied,
    COUNT(*) FILTER (WHERE application_status = $5)                             AS total_interviewing,
    COUNT(*) FILTER (WHERE application_status = $6)                             AS total_won,
    COUNT(*) FILTER (WHERE application_status = $7)                             AS total_lost
FROM jobs
WHERE user_id = $1`

	var st UserJobStats
	err := s.db.QueryRowContext(ctx, q,
		userID,
		string(models.StatusApproved),
		string(models.StatusRejected),
		string(models.AppStatusApplied),
		string(models.AppStatusInterviewing),
		string(models.AppStatusWon),
		string(models.AppStatusLost),
	).Scan(
		&st.TotalFound,
		&st.TotalApproved,
		&st.TotalRejected,
		&st.TotalApplied,
		&st.TotalInterviewing,
		&st.TotalWon,
		&st.TotalLost,
	)
	if err != nil {
		return st, fmt.Errorf("store: get user job stats: %w", err)
	}
	return st, nil
}

// GetJobsPerWeek returns the number of jobs discovered per calendar week for
// the given user, looking back the specified number of weeks. Results are
// ordered by week_start ascending. Weeks with no jobs are omitted.
func (s *Store) GetJobsPerWeek(ctx context.Context, userID int64, weeks int) ([]WeeklyJobCount, error) {
	const q = `
SELECT
    date_trunc('week', discovered_at AT TIME ZONE 'UTC') AS week_start,
    COUNT(*)                                              AS cnt
FROM jobs
WHERE user_id = $1
  AND discovered_at >= NOW() - ($2 * INTERVAL '1 week')
GROUP BY week_start
ORDER BY week_start ASC`

	rows, err := s.db.QueryContext(ctx, q, userID, weeks)
	if err != nil {
		return nil, fmt.Errorf("store: get jobs per week: %w", err)
	}
	defer rows.Close()

	var result []WeeklyJobCount
	for rows.Next() {
		var wc WeeklyJobCount
		if err := rows.Scan(&wc.WeekStart, &wc.Count); err != nil {
			return nil, fmt.Errorf("store: get jobs per week scan: %w", err)
		}
		result = append(result, wc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: get jobs per week rows: %w", err)
	}
	return result, nil
}
