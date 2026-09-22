// Package digest emails each opted-in profile the new jobs that match its
// active resume.
package digest

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"main/database"
	"main/jobs"
	"main/notify"
	"main/relevance"
	"main/schedule"
)

const interval = 24 * time.Hour
const jobLimit = 10
const goodFitRatio = 0.4
const firstRunLookback = 24 * time.Hour
const runTimeout = 2 * time.Minute

const jobName = "digest"

// StartScheduler sends the digest once at startup, then once per interval
// until ctx is cancelled. Run it in its own goroutine.
func StartScheduler(ctx context.Context) {
	schedule.Every(ctx, jobName, interval, run)
}

// RunCycle runs one digest pass across all opted-in profiles, for cmd/cron.
func RunCycle() {
	schedule.Once(jobName, run)
}

func run() {
	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()

	profiles, err := database.ListProfilesForDigest(ctx)

	if err != nil {
		slog.Error("digest: list profiles", "error", err)
		return
	}

	for _, profile := range profiles {
		sendForProfileSafely(ctx, profile)
	}
}

// sendForProfileSafely isolates one profile's digest from the rest of the
// batch. A panic building/sending one profile's email shouldn't skip every
// other profile in this run.
func sendForProfileSafely(ctx context.Context, profile database.DigestProfile) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("digest: profile: panic", "profile_id", profile.ProfileID, "panic", r)
		}
	}()

	if err := sendForProfile(ctx, profile); err != nil {
		slog.Error("digest: profile", "profile_id", profile.ProfileID, "error", err)
	}
}

func sendForProfile(ctx context.Context, profile database.DigestProfile) error {
	extraction, found, err := database.GetActiveResumeExtraction(ctx, profile.ProfileID)

	if err != nil {
		return fmt.Errorf("get active resume extraction: %w", err)
	}

	if !found {
		return nil
	}

	resumeQuery := relevance.SearchQuery(extraction)

	if resumeQuery == "" {
		return nil
	}

	windowStart := time.Now().Add(-firstRunLookback)
	if profile.LastDigestSentAt != nil {
		windowStart = *profile.LastDigestSentAt
	}

	result, err := database.SearchForJobs(ctx, &database.JobSearchParams{
		ResumeQuery:     resumeQuery,
		ResumeEmbedding: extraction.Embedding,
		PostedAfter:     &windowStart,
		Sort:            database.SortRelevance,
		Limit:           jobLimit,
	})

	if err != nil {
		return fmt.Errorf("search jobs: %w", err)
	}

	matched := goodFitJobs(result.Jobs)
	sentAt := time.Now()

	if len(matched) > 0 {
		if err := notify.SendEmail(ctx, profile.Email, subject(len(matched)), renderEmail(matched, profile.ProfileID)); err != nil {
			return fmt.Errorf("send email: %w", err)
		}
	}

	if err := database.MarkDigestSent(ctx, profile.ProfileID, sentAt); err != nil {
		return fmt.Errorf("mark digest sent: %w", err)
	}

	return nil
}

// goodFitJobs keeps jobs scoring at least goodFitRatio of the batch's best
// score, the same relative tiering the "Good fit" badge uses client-side,
// since a raw ts_rank score isn't on any fixed scale.
func goodFitJobs(candidates []jobs.Job) []jobs.Job {
	var best float64

	for _, job := range candidates {
		if job.MatchScore != nil && *job.MatchScore > best {
			best = *job.MatchScore
		}
	}

	if best <= 0 {
		return nil
	}

	var matched []jobs.Job

	for _, job := range candidates {
		if job.MatchScore != nil && *job.MatchScore/best >= goodFitRatio {
			matched = append(matched, job)
		}
	}

	return matched
}

func subject(count int) string {
	if count == 1 {
		return "1 new job matches your resume"
	}

	return fmt.Sprintf("%d new jobs match your resume", count)
}

func renderEmail(matched []jobs.Job, profileID int64) string {
	var b strings.Builder

	b.WriteString("New jobs that match your resume:\n\n")

	for _, job := range matched {
		fmt.Fprintf(&b, "%s at %s\n%s\n\n", job.Title, job.Company, job.URL)
	}

	fmt.Fprintf(&b, "---\nDon't want these emails? Unsubscribe: %s\n", unsubscribeLink(profileID))

	return b.String()
}
