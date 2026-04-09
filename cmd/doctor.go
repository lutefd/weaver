package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/lutefd/weaver/internal/doctor"
	gitrunner "github.com/lutefd/weaver/internal/git"
	"github.com/lutefd/weaver/internal/ui"
	"github.com/spf13/cobra"
)

func init() {
	doctorCmd.Flags().Bool("json", false, "print the doctor report as JSON")
	rootCmd.AddCommand(doctorCmd)
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Inspect repository and Weaver state for common problems",
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := context.Background()
		asJSON, err := cmd.Flags().GetBool("json")
		if err != nil {
			return err
		}

		report, err := runTask(ctx, cmd, ui.TaskSpec{
			Title:    "Running Doctor",
			Subtitle: "Checking repository and Weaver state",
		}, func(ctx context.Context, runner gitrunner.Runner) (*doctor.Report, error) {
			return doctor.New(runner, AppContext().Config, AppContext().ConfigErr).Run(ctx)
		})
		if err != nil {
			return err
		}

		if asJSON {
			encoder := json.NewEncoder(cmd.OutOrStdout())
			encoder.SetIndent("", "  ")
			if err := encoder.Encode(report); err != nil {
				return err
			}
		} else {
			term := terminalFor(cmd)
			if term.Styled() {
				writeLine(cmd.OutOrStdout(), renderDoctorReportStyled(term, report))
			} else {
				renderDoctorReport(cmd.OutOrStdout(), report)
			}
		}

		if report.HasFailures() {
			return fmt.Errorf("doctor found %d failure(s)", report.Summary.Fail)
		}

		return nil
	},
}

func renderDoctorReport(w io.Writer, report *doctor.Report) {
	for _, check := range report.Checks {
		fmt.Fprintf(w, "%-4s %s\n", strings.ToUpper(string(check.Level)), check.Message)
		if check.Hint != "" {
			fmt.Fprintf(w, "     fix: %s\n", check.Hint)
		}
	}

	fmt.Fprintf(w, "summary: %d ok, %d warn, %d fail\n", report.Summary.OK, report.Summary.Warn, report.Summary.Fail)
}
