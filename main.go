package main

import (
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2/jwt"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"google.golang.org/api/option"
	playdeveloperreporting "google.golang.org/api/playdeveloperreporting/v1beta1"
	"image/color"
	"image/png"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	GoogleApplicationCredentials = "./service-account.json"
	ProjectID                    = "apps/level.game"
)

type WeeklyAvg struct {
	Week  int     `json:"week"`
	Value float64 `json:"value"`
}
type CrashData struct {
	CrashRate string `json:"crash_rate"`
	Date      string `json:"date"`
}

func main() {

	http.HandleFunc("/get-crashes", getWeeklyCrashRate)
	http.HandleFunc("/movingavg/crashes", getWeeklyAvgComparison)
	fmt.Println("Listening on port 8080")

	http.ListenAndServe(":8080", nil)

	fmt.Println("Listening on port 8080")

}

func getWeeklyCrashRate(
	w http.ResponseWriter,
	r *http.Request,
) {
	tokenSource, err := getTokenSource(GoogleApplicationCredentials)
	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	service, err := playdeveloperreporting.NewService(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return
	}

	startTime := time.Now().AddDate(0, 0, -8)
	endTime := time.Now().AddDate(0, 0, -1)

	startDateTime := &playdeveloperreporting.GoogleTypeDateTime{
		Year:  int64(startTime.Year()),
		Month: int64(startTime.Month()),
		Day:   int64(startTime.Day()),
	}

	endDateTime := &playdeveloperreporting.GoogleTypeDateTime{
		Year:  int64(endTime.Year()),
		Month: int64(endTime.Month()),
		Day:   int64(endTime.Day()),
	}

	timelineSpec := &playdeveloperreporting.GooglePlayDeveloperReportingV1beta1TimelineSpec{
		StartTime:         startDateTime,
		EndTime:           endDateTime,
		AggregationPeriod: "DAILY",
	}

	crashes := service.Vitals.Crashrate.Query(
		ProjectID+"/crashRateMetricSet", &playdeveloperreporting.GooglePlayDeveloperReportingV1beta1QueryCrashRateMetricSetRequest{
			TimelineSpec: timelineSpec,
			Metrics:      []string{"crashRate"},
		},
	)

	result, err := crashes.Do()
	if err != nil {
		return
	}
	var crashDataList []CrashData

	// Process each row in the result
	for _, row := range result.Rows {
		crashData := formatCrashData(row)
		crashDataList = append(crashDataList, crashData)
	}

	xys := make(plotter.XYs, len(crashDataList))

	// Print the formatted crash data
	for i, data := range crashDataList {
		parsedDate, err := time.Parse("02/01/2006", data.Date)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		rateWithoutPercent := strings.TrimSuffix(data.CrashRate, "%")
		crashRate, err := strconv.ParseFloat(rateWithoutPercent, 64)
		xys[i].X = float64(parsedDate.Unix()) // Use Unix timestamp for X-axis
		xys[i].Y = crashRate
	}

	// Create a new plot
	p := plot.New()

	// Create a line plot
	line, err := plotter.NewLine(xys)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	line.Color = color.RGBA{R: 0, G: 0, B: 255, A: 255} // Line color
	p.Add(line)

	// Set the plot title and labels
	p.Title.Text = "Crash Rate Over Time"
	p.X.Label.Text = "Date"
	p.Y.Label.Text = "Crash Rate (%)"

	// Format X-axis to show dates properly p.X.Tick.Label.
	p.X.Tick.Marker = plot.TimeTicks{Format: "02/01/2006"}

	w.Header().Set("Content-Type", "image/png")

	// Save the plot as a PNG image
	if err := p.Save(10*vg.Inch, 5*vg.Inch, "crash_rate_plot.png"); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	imgFile, err := os.Open("crash_rate_plot.png")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer imgFile.Close()

	// Decode the image
	img, err := png.Decode(imgFile)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Write the image to the response
	if err := png.Encode(w, img); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	return
}

func getWeeklyAvgComparison(
	w http.ResponseWriter,
	r *http.Request,
) {

	var weeklyAvg []WeeklyAvg

	tokenSource, err := getTokenSource(GoogleApplicationCredentials)

	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	service, err := playdeveloperreporting.NewService(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		panic(err)
		return
	}
	previousStartTime := time.Now().AddDate(0, 0, -15)
	previousEndTime := time.Now().AddDate(0, 0, -8)

	previousStartDateTime := &playdeveloperreporting.GoogleTypeDateTime{
		Year:  int64(previousStartTime.Year()),
		Month: int64(previousStartTime.Month()),
		Day:   int64(previousStartTime.Day()),
	}

	previousEndDateTime := &playdeveloperreporting.GoogleTypeDateTime{
		Year:  int64(previousEndTime.Year()),
		Month: int64(previousEndTime.Month()),
		Day:   int64(previousEndTime.Day()),
	}

	previousTimeSpec := &playdeveloperreporting.GooglePlayDeveloperReportingV1beta1TimelineSpec{
		StartTime:         previousStartDateTime,
		EndTime:           previousEndDateTime,
		AggregationPeriod: "DAILY",
	}

	previousCrashes := service.Vitals.Crashrate.Query(
		ProjectID+"/crashRateMetricSet", &playdeveloperreporting.GooglePlayDeveloperReportingV1beta1QueryCrashRateMetricSetRequest{
			TimelineSpec: previousTimeSpec,
			Metrics:      []string{"crashRate7dUserWeighted"},
		},
	)
	previousResult, err := previousCrashes.Do()
	if err != nil {
		fmt.Println(err)
		return
	}

	// Process each row in the result
	if len(previousResult.Rows) > 0 {
		lastRow := previousResult.Rows[len(previousResult.Rows)-1]
		fmt.Printf("Start Time: %v\n", lastRow.StartTime)
		for _, metricValue := range lastRow.Metrics {
			if metricValue.Metric == "crashRate7dUserWeighted" {
				data := WeeklyAvg{
					Week:  0,
					Value: parseDecimal(metricValue.DecimalValue.Value) * 100,
				}
				weeklyAvg = append(weeklyAvg, data)
			}
		}
	} else {
		fmt.Println("No data available for the specified period.")
	}

	startTime := time.Now().AddDate(0, 0, -8)
	endTime := time.Now().AddDate(0, 0, -1)

	startDateTime := &playdeveloperreporting.GoogleTypeDateTime{
		Year:  int64(startTime.Year()),
		Month: int64(startTime.Month()),
		Day:   int64(startTime.Day()),
	}

	endDateTime := &playdeveloperreporting.GoogleTypeDateTime{
		Year:  int64(endTime.Year()),
		Month: int64(endTime.Month()),
		Day:   int64(endTime.Day()),
	}

	timelineSpec := &playdeveloperreporting.GooglePlayDeveloperReportingV1beta1TimelineSpec{
		StartTime:         startDateTime,
		EndTime:           endDateTime,
		AggregationPeriod: "DAILY",
	}

	crashes := service.Vitals.Crashrate.Query(
		ProjectID+"/crashRateMetricSet", &playdeveloperreporting.GooglePlayDeveloperReportingV1beta1QueryCrashRateMetricSetRequest{
			TimelineSpec: timelineSpec,
			Metrics:      []string{"crashRate7dUserWeighted"},
		},
	)
	result, err := crashes.Do()
	if err != nil {
		fmt.Println(err)
		return
	}
	//	var crashDataList []CrashData

	// Process each row in the result
	if len(result.Rows) > 0 {
		lastRow := result.Rows[len(result.Rows)-1]
		fmt.Printf("Start Time: %v\n", lastRow.StartTime)
		for _, metricValue := range lastRow.Metrics {
			if metricValue.Metric == "crashRate7dUserWeighted" {
				data := WeeklyAvg{
					Week:  1,
					Value: parseDecimal(metricValue.DecimalValue.Value) * 100,
				}
				weeklyAvg = append(weeklyAvg, data)
			}
		}
	} else {
		fmt.Println("No data available for the specified period.")
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(weeklyAvg); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

}

func getTokenSource(credentialFile string) (oauth2.TokenSource, error) {
	ctx := context.Background()
	b, err := os.ReadFile(credentialFile)
	if err != nil {
		return nil, err
	}
	var c = struct {
		Email      string `json:"client_email"`
		PrivateKey string `json:"private_key"`
	}{}
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	fmt.Printf("\nClient email: %s\n", c.Email)
	config := &jwt.Config{
		Email:      c.Email,
		PrivateKey: []byte(c.PrivateKey),
		Scopes: []string{
			"https://www.googleapis.com/auth/playdeveloperreporting",
		},
		TokenURL: google.JWTTokenURL,
	}
	return config.TokenSource(ctx), nil
}

func formatCrashData(row *playdeveloperreporting.GooglePlayDeveloperReportingV1beta1MetricsRow) CrashData {
	var crashRate string
	var date string

	for _, metric := range row.Metrics {
		if metric.Metric == "crashRate" {
			// Calculate crash rate as a percentage
			rateValue := metric.DecimalValue.Value
			crashRate = fmt.Sprintf("%.2f%%", parseDecimal(rateValue)*100)

			// Format date
			startTime := row.StartTime
			date = fmt.Sprintf("%02d/%02d/%d", startTime.Day, startTime.Month, startTime.Year)
		}
	}

	return CrashData{
		CrashRate: crashRate,
		Date:      date,
	}
}

// Helper function to convert string to float64
func parseDecimal(value string) float64 {
	var result float64
	fmt.Sscanf(value, "%f", &result)
	return result
}
