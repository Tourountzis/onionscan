package steps

import (
	"bytes"
	"fmt"
	"github.com/s-rah/onionscan/report"
	"log"
	"os"
	"time"
)

type JsonReportWriter struct {
	reportFile string
}

func (jrw *JsonReportWriter) Init(outputFile string) {
	jrw.reportFile = outputFile
}

func (jrw *JsonReportWriter) Do(r *report.OnionScanReport) error {
	jsonOut, err := r.Serialize()

	if err != nil {
		return err
	}

	var buffer bytes.Buffer

	buffer.WriteString(fmt.Sprintf("%s\n", jsonOut))

	reportFile := r.HiddenService + "." + jrw.reportFile

	if len(reportFile) > 0 {
		f, err := os.Create(reportFile)

		for err != nil {
			log.Printf("Cannot create report file: %s...trying again in 5 seconds...", err)
			time.Sleep(time.Second * 5)
			f, err = os.Create(reportFile)
		}

		defer f.Close()

		f.WriteString(buffer.String())
	} else {
		fmt.Print(buffer.String())
	}
	return nil
}
