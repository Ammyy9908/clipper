// Command purge prunes stuck work out of the download queue.
//
// It only looks unless told to act:
//
//	purge                          show what a purge would remove
//	purge --apply                  do it
//	purge --older-than 2h          only jobs idle for at least 2 hours (default 30m)
//	purge --user <id>              only one user's or device's jobs
//	purge --clear-finished         also drop completed/archived task history
//
// It removes queue tasks that belong to stuck jobs, or whose job is already
// finished or gone, and marks the stuck jobs failed. Jobs a worker is running
// right now are left alone.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/url"
	"time"

	"github.com/hibiken/asynq"

	"github.com/ammyy9908/clipper/internal/config"
	"github.com/ammyy9908/clipper/internal/purge"
	"github.com/ammyy9908/clipper/internal/queue"
	"github.com/ammyy9908/clipper/internal/store"
)

const maxListed = 25

func main() {
	apply := flag.Bool("apply", false, "make the changes (default is a dry run)")
	olderThan := flag.Duration("older-than", 30*time.Minute, "how long a job must be idle to count as stuck")
	user := flag.String("user", "", "only purge this user or device id")
	clearFinished := flag.Bool("clear-finished", false, "also delete completed and archived task history")
	flag.Parse()

	cfg := config.Load()
	ctx := context.Background()

	redisOpt, err := queue.ParseRedisOpt(cfg.RedisAddr)
	if err != nil {
		log.Fatalf("parse redis address: %v", err)
	}
	inspector := asynq.NewInspector(redisOpt)
	defer inspector.Close()

	db, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	// Say which environment this is before anything else, so a purge pointed at
	// the wrong place is obvious. Credentials are never printed.
	fmt.Printf("Database: %s\nRedis:    %s\n", host(cfg.DatabaseURL), host(cfg.RedisAddr))
	fmt.Printf("Looking for jobs idle for %s or more", *olderThan)
	if *user != "" {
		fmt.Printf(", for user %q", *user)
	}
	fmt.Print("\n\n")

	q := purge.Asynq{In: inspector}
	opt := purge.Options{OlderThan: *olderThan, UserID: *user, ClearFinished: *clearFinished}

	plan, err := purge.BuildPlan(ctx, q, db, opt)
	if err != nil {
		log.Fatalf("could not build a plan, nothing was changed: %v", err)
	}
	printPlan(plan)

	if plan.Empty() {
		fmt.Println("Nothing to purge.")
		return
	}
	if !*apply {
		fmt.Println("\nDry run: nothing was changed. Re-run with --apply to do this.")
		return
	}

	res, err := purge.Apply(ctx, q, db, plan, opt)
	fmt.Printf("\nDeleted %d queue tasks (%d could not be deleted), cancelled %d jobs",
		res.TasksDeleted, res.TasksSkipped, res.JobsCancelled)
	if res.JobsKept > 0 {
		fmt.Printf(", left %d alone because their task could not be removed", res.JobsKept)
	}
	fmt.Println(".")
	if err != nil {
		log.Fatalf("purge stopped early: %v", err)
	}
}

func printPlan(p purge.Plan) {
	fmt.Printf("Stuck jobs to cancel: %d\n", len(p.Cancel))
	for i, j := range p.Cancel {
		if i == maxListed {
			fmt.Printf("  ... and %d more\n", len(p.Cancel)-maxListed)
			break
		}
		fmt.Printf("  %s  %-11s idle %-7s %s\n", j.ID.String()[:8], j.Status, idle(j.UpdatedAt), label(j))
	}

	if len(p.SkippedActive) > 0 {
		fmt.Printf("\nLeft alone, a worker is running them now: %d\n", len(p.SkippedActive))
		for i, j := range p.SkippedActive {
			if i == maxListed {
				fmt.Printf("  ... and %d more\n", len(p.SkippedActive)-maxListed)
				break
			}
			fmt.Printf("  %s  %-11s idle %-7s %s\n", j.ID.String()[:8], j.Status, idle(j.UpdatedAt), label(j))
		}
	}

	fmt.Printf("\nQueue tasks to delete: %d\n", len(p.Tasks))
	byReason := map[string]int{}
	for _, t := range p.Tasks {
		byReason[t.Reason]++
	}
	for _, reason := range []string{purge.ReasonStuck, purge.ReasonFinished, purge.ReasonMissing} {
		if n := byReason[reason]; n > 0 {
			fmt.Printf("  %d because %s\n", n, reason)
		}
	}
	if p.Unreadable > 0 {
		fmt.Printf("  (%d tasks have an unreadable payload and will not be touched)\n", p.Unreadable)
	}

	if p.FinishedCompleted+p.FinishedArchived > 0 {
		fmt.Printf("\nTask history to clear: %d completed, %d archived\n", p.FinishedCompleted, p.FinishedArchived)
	}
}

func label(j store.UnfinishedJob) string {
	if j.Title != nil && *j.Title != "" {
		return *j.Title
	}
	if j.SourceURL != nil {
		return *j.SourceURL
	}
	return "(no title yet)"
}

func idle(t time.Time) string {
	d := time.Since(t).Round(time.Minute)
	if d >= 24*time.Hour {
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
	if d >= time.Hour {
		return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
	}
	return fmt.Sprintf("%dm", int(d.Minutes()))
}

// host returns just the host[:port] of a connection string, dropping credentials
// and paths. Bare host:port values pass through.
func host(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return raw
	}
	return u.Host
}
