package server

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"main/database"
	"main/jobs"
	"main/notify"
)

const digestInterval = 24 * time.Hour
const digestJobLimit = 10
const digestGoodFitRatio = 0.4
const digestFirstRunLookback = 24 * time.Hour
const digestJobTimeout = 2 * time.Minute

// StartDigestScheduler runs the job-match email digest once at startup, then once
// per digestInterval. Meant to be launched in its own goroutine, mirroring
// scraper.StartScrapingJob.
func StartDigestScheduler() {
	runDigestJobSafely()

	ticker := time.NewTicker(digestInterval)
	defer ticker.Stop()

	for range ticker.C {
		runDigestJobSafely()
	}
}

// runDigestJobSafely wraps runDigestJob with a panic recovery so one bad run
// logs and moves on instead of killing the scheduler goroutine (and, since
// nothing restarts it, silently ending all future digests) for the rest of
// the process's life.
func runDigestJobSafely() {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("digest: panic during run", "panic", r)
		}
	}()

	runDigestJob()
}

func runDigestJob() {
	ctx, cancel := context.WithTimeout(context.Background(), digestJobTimeout)
	defer cancel()

	profiles, err := database.ListProfilesForDigest(ctx)

	if err != nil {
		slog.Error("digest: list profiles", "error", err)
		return
	}

	for _, profile := range profiles {
		sendDigestForProfileSafely(ctx, profile)
	}
}

// sendDigestForProfileSafely isolates one profile's digest from the rest of the
// batch — a panic building/sending one profile's email shouldn't skip every
// other profile in this run.
func sendDigestForProfileSafely(ctx context.Context, profile database.DigestProfile) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("digest: profile: panic", "profile_id", profile.ProfileID, "panic", r)
		}
	}()

	if err := sendDigestForProfile(ctx, profile); err != nil {
		slog.Error("digest: profile", "profile_id", profile.ProfileID, "error", err)
	}
}

func sendDigestForProfile(ctx context.Context, profile database.DigestProfile) error {
	extraction, found, err := database.GetActiveResumeExtraction(ctx, profile.ProfileID)

	if err != nil {
		return fmt.Errorf("get active resume extraction: %w", err)
	}

	if !found {
		return nil
	}

	resumeQuery := buildResumeSearchQuery(extraction)

	if resumeQuery == "" {
		return nil
	}

	windowStart := time.Now().Add(-digestFirstRunLookback)
	if profile.LastDigestSentAt != nil {
		windowStart = *profile.LastDigestSentAt
	}

	result, err := database.SearchForJobs(ctx, &database.JobSearchParams{
		ResumeQuery: resumeQuery,
		PostedAfter: &windowStart,
		Sort:        database.SortRelevance,
		Limit:       digestJobLimit,
	})

	if err != nil {
		return fmt.Errorf("search jobs: %w", err)
	}

	matched := goodFitJobs(result.Jobs)
	sentAt := time.Now()

	if len(matched) > 0 {
		if err := notify.SendEmail(ctx, profile.Email, digestSubject(len(matched)), renderDigestEmail(matched)); err != nil {
			return fmt.Errorf("send email: %w", err)
		}
	}

	if err := database.MarkDigestSent(ctx, profile.ProfileID, sentAt); err != nil {
		return fmt.Errorf("mark digest sent: %w", err)
	}

	return nil
}

// goodFitJobs keeps jobs whose match score is at least digestGoodFitRatio of the
// best score in this batch — the same relative-tiering the "Good fit" badge uses
// client-side, since a raw ts_rank score isn't calibrated to an absolute scale.
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
		if job.MatchScore != nil && *job.MatchScore/best >= digestGoodFitRatio {
			matched = append(matched, job)
		}
	}

	return matched
}

func digestSubject(count int) string {
	if count == 1 {
		return "1 new job matches your resume"
	}

	return fmt.Sprintf("%d new jobs match your resume", count)
}

func renderDigestEmail(matched []jobs.Job) string {
	var b strings.Builder

	b.WriteString("New jobs that match your resume:\n\n")

	for _, job := range matched {
		fmt.Fprintf(&b, "%s at %s\n%s\n\n", job.Title, job.Company, job.URL)
	}

	return b.String()
}
