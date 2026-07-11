// narrativeqa validates authored narrative content and writes a CI artifact.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"animalpoke/backend/internal/narrativeqa"
)

func main() {
	reportPath := flag.String("report", "", "path for the JSON QA report; stdout when empty")
	flag.Parse()

	report := narrativeqa.AnalyzeSeed()
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "encode narrative QA report:", err)
		os.Exit(2)
	}
	encoded = append(encoded, '\n')
	if *reportPath == "" {
		_, err = os.Stdout.Write(encoded)
	} else {
		err = os.WriteFile(*reportPath, encoded, 0o600)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "write narrative QA report:", err)
		os.Exit(2)
	}
	if !report.Valid() {
		fmt.Fprintf(os.Stderr, "narrative QA gate failed (%d diagnostics)\n", len(report.Diagnostics))
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "narrative QA gate OK (%d paths, %d endings)\n", len(report.Paths), len(report.Endings))
}
