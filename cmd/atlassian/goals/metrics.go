package goals

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/jinkp/atlassian-go-mcp/internal/audit"
	"github.com/jinkp/atlassian-go-mcp/internal/atlassian/goals"
	"github.com/jinkp/atlassian-go-mcp/internal/cliutil"
	"github.com/spf13/cobra"
)

// NewMetricsCmd returns the "metrics <goal-id>" command that lists all MetricTargets for a goal.
func NewMetricsCmd(svc goals.GoalsService) *cobra.Command {
	return &cobra.Command{
		Use:   "metrics <goal-id>",
		Short: "List all metric targets for an Atlassian Goal",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			goalID := args[0]

			metrics, err := svc.GetGoalMetrics(context.Background(), goalID)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(goalsExitCode(err))
			}

			data, err := json.MarshalIndent(metrics, "", "  ")
			if err != nil {
				fmt.Fprintln(os.Stderr, "error formatting output:", err)
				os.Exit(2)
			}

			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}
}

// NewMetricCreateCmd returns the "metric-create" command that creates a new metric on a goal.
func NewMetricCreateCmd(svc goals.GoalsService, auditLog audit.Logger, dryRun bool) *cobra.Command {
	var (
		goalID       string
		name         string
		metricType   string
		startValue   float64
		targetValue  float64
		initialValue float64
	)

	cmd := &cobra.Command{
		Use:   "metric-create",
		Short: "Create a new metric and attach it to an Atlassian Goal",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.ResolveDryRun(cmd, dryRun) {
				fmt.Fprintf(cmd.OutOrStdout(), "[DRY RUN] Would create metric: goal-id=%s name=%q type=%s start=%g target=%g initial=%g\n",
					goalID, name, metricType, startValue, targetValue, initialValue)
				return nil
			}

			req := goals.CreateMetricRequest{
				GoalID:       goalID,
				Name:         name,
				MetricType:   metricType,
				StartValue:   startValue,
				TargetValue:  targetValue,
				InitialValue: initialValue,
			}

			result, err := svc.CreateMetric(context.Background(), req)
			auditLog.Log(audit.NewEntry("create_metric", "goals",
				map[string]any{"goal_id": goalID, "name": name}, err))
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(goalsExitCode(err))
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Created metric target: %s (metric: %s %s)\n",
				result.ID, result.Metric.ID, result.Metric.Name)
			return nil
		},
	}

	cmd.Flags().StringVar(&goalID, "goal-id", "", "Goal ARI (required)")
	cmd.Flags().StringVar(&name, "name", "", "Metric name (required)")
	cmd.Flags().StringVar(&metricType, "type", "", "Metric type: CURRENCY | NUMERIC | PERCENTAGE (required)")
	cmd.Flags().Float64Var(&startValue, "start", 0, "Start value (required)")
	cmd.Flags().Float64Var(&targetValue, "target", 0, "Target value (required)")
	cmd.Flags().Float64Var(&initialValue, "initial", 0, "Initial current value (required)")
	_ = cmd.MarkFlagRequired("goal-id")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("type")
	_ = cmd.MarkFlagRequired("start")
	_ = cmd.MarkFlagRequired("target")
	_ = cmd.MarkFlagRequired("initial")
	return cmd
}

// NewMetricValueCmd returns the "metric-value" command that adds a value datapoint to a metric.
func NewMetricValueCmd(svc goals.GoalsService, auditLog audit.Logger, dryRun bool) *cobra.Command {
	var (
		metricID string
		value    float64
		timeStr  string
	)

	cmd := &cobra.Command{
		Use:   "metric-value",
		Short: "Add a value datapoint to an existing metric",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.ResolveDryRun(cmd, dryRun) {
				fmt.Fprintf(cmd.OutOrStdout(), "[DRY RUN] Would add metric value: metric-id=%s value=%g\n", metricID, value)
				return nil
			}

			req := goals.UpdateMetricValueRequest{
				MetricID: metricID,
				Value:    value,
				Time:     timeStr,
			}

			result, err := svc.UpdateMetricValue(context.Background(), req)
			auditLog.Log(audit.NewEntry("update_metric_value", "goals",
				map[string]any{"metric_id": metricID, "value": value}, err))
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(goalsExitCode(err))
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Added metric value: id=%s value=%g\n", result.ID, result.Value)
			return nil
		},
	}

	cmd.Flags().StringVar(&metricID, "metric-id", "", "Metric ID (required)")
	cmd.Flags().Float64Var(&value, "value", 0, "Value to record (required)")
	cmd.Flags().StringVar(&timeStr, "time", "", "Timestamp ISO 8601, e.g. 2024-01-15T00:00:00Z (optional)")
	_ = cmd.MarkFlagRequired("metric-id")
	_ = cmd.MarkFlagRequired("value")
	return cmd
}

// NewMetricTargetCmd returns the "metric-target" command that updates MetricTarget values.
func NewMetricTargetCmd(svc goals.GoalsService, auditLog audit.Logger, dryRun bool) *cobra.Command {
	var (
		metricTargetID string
		currentValue   float64
		startValue     float64
		targetValue    float64
	)

	cmd := &cobra.Command{
		Use:   "metric-target",
		Short: "Update start, current, and/or target values of a MetricTarget",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.ResolveDryRun(cmd, dryRun) {
				fmt.Fprintf(cmd.OutOrStdout(), "[DRY RUN] Would update metric target: metric-target-id=%s\n", metricTargetID)
				return nil
			}

			req := goals.UpdateMetricTargetRequest{
				MetricTargetID: metricTargetID,
			}
			if cmd.Flags().Changed("current") {
				req.CurrentValue = &currentValue
			}
			if cmd.Flags().Changed("start") {
				req.StartValue = &startValue
			}
			if cmd.Flags().Changed("target") {
				req.TargetValue = &targetValue
			}

			err := svc.UpdateMetricTarget(context.Background(), req)
			auditLog.Log(audit.NewEntry("update_metric_target", "goals",
				map[string]any{"metric_target_id": metricTargetID}, err))
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(goalsExitCode(err))
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Updated metric target: %s\n", metricTargetID)
			return nil
		},
	}

	cmd.Flags().StringVar(&metricTargetID, "metric-target-id", "", "MetricTarget ID (required)")
	cmd.Flags().Float64Var(&currentValue, "current", 0, "New current value (optional)")
	cmd.Flags().Float64Var(&startValue, "start", 0, "New start value (optional)")
	cmd.Flags().Float64Var(&targetValue, "target", 0, "New target value (optional)")
	_ = cmd.MarkFlagRequired("metric-target-id")
	return cmd
}
