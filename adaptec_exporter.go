// Copyright 2015 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	collector_version "github.com/prometheus/client_golang/prometheus/collectors/version"
	"github.com/tidwall/gjson"
	"strconv"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promlog"
	"github.com/prometheus/common/promlog/flag"
	"github.com/prometheus/exporter-toolkit/web"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/common/version"
	webflag "github.com/prometheus/exporter-toolkit/web/kingpinflag"
)

const (
	namespace                  = "adaptec"
	arcconfMinimalBuildVersion = 26540
)

type Exporter struct {
	logger log.Logger

	rw              *sync.RWMutex
	executablePath  string
	minimalInterval time.Duration

	controllerCount int

	collectSMART            bool
	skipCollectSMARTForJDOB bool

	CmdStdout                map[string][]byte
	CmdStderr                map[string][]byte
	CmdErr                   map[string]error
	CommandLastedExecuteTime map[string]time.Time
}

type PhysicalDriveSmartStatsAttribute struct {
	Id                string `xml:"id,attr"`
	Name              string `xml:"name,attr"`
	NormalizedCurrent int    `xml:"normalizedCurrent,attr"`
	NormalizedWorst   int    `xml:"normalizedWorst,attr"`
	ThresholdValue    int    `xml:"thresholdValue,attr"`
	RawValue          int    `xml:"rawValue,attr"`
	Value             string `xml:"Value,attr"` // for scsi, it maybe a string
	Status            string `xml:"Status,attr"`
}

type PhysicalDriveSmartStats struct {
	Channel                int                                `xml:"channel,attr"`
	Id                     int                                `xml:"id,attr"`
	NonSpinning            bool                               `xml:"nonSpinning,attr"`
	IsDescriptionAvailable bool                               `xml:"isDescriptionAvailable,attr"`
	Attribute              []PhysicalDriveSmartStatsAttribute `xml:"Attribute"`
}

type SmartStats struct {
	ControllerId  string                    `xml:"controllerID,attr"`
	Timestamp     int64                     `xml:"time,attr"`
	DeviceName    string                    `xml:"deviceName,attr"`
	SerialNumber  string                    `xml:"serialNumber,attr"`
	PhySmartStats []PhysicalDriveSmartStats `xml:"PhysicalDriveSmartStats"`
}

func NewExporter(execBin string, extraLookupPath []string, miniExecInterval time.Duration,
	enableSMART, skipCollectSMARTForJDOB bool, logger log.Logger) (*Exporter, error) {
	if execBin == "" {
		level.Error(logger).Log("msg", "exec bin is empty")
		return nil, fmt.Errorf("exec bin is empty")
	}

	os.Setenv("PATH", strings.Join(extraLookupPath, string(os.PathListSeparator)))

	executablePath, err := exec.LookPath(execBin)
	if err != nil {
		level.Error(logger).Log("msg", "exec bin not found", "err", err)
		return nil, fmt.Errorf("exec bin not found")
	}

	if miniExecInterval < 60*time.Second {
		level.Warn(logger).Log("msg", "minimal execute interval is less than 60s, it's dangerous!")
	}

	versionCmd := exec.Command(executablePath, "VERSION")
	stdout := &strings.Builder{}
	versionCmd.Stdout = stdout

	err = versionCmd.Run()
	if err != nil {
		level.Error(logger).Log("msg", "exec bin run get version failed", "err", err)
		return nil, err
	}
	level.Debug(logger).Log("stdout", stdout.String())

	versionOutputArray := strings.Split(stdout.String(), "\n")
	if len(versionOutputArray) < 4 {
		level.Error(logger).Log("msg", "exec bin run get version failed", "err", "output is too short")
		return nil, fmt.Errorf("exec bin run get version failed")
	}

	var unusedVersion string
	var controllerCount, buildNumber int

	_, err = fmt.Sscanf(versionOutputArray[0], "Controllers found: %d", &controllerCount)
	if err != nil {
		level.Error(logger).Log("msg", "exec bin run get version failed", "err", err)
		return nil, fmt.Errorf("exec bin run get version failed")
	}
	if controllerCount < 1 {
		level.Error(logger).Log("err", "no controller be found")
		return nil, fmt.Errorf("no controller be found")
	}

	n, err := fmt.Sscanf(strings.TrimSpace(versionOutputArray[3]), "| UCLI |  Version %s (%d)", &unusedVersion, &buildNumber)
	if err != nil {
		level.Error(logger).Log("msg", "exec bin run get version failed", "err", err)
		return nil, fmt.Errorf("exec bin run get version failed")
	}
	if n != 2 {
		level.Error(logger).Log("msg", "exec bin run get version failed", "err", "output format error")
		return nil, fmt.Errorf("exec bin run get version failed")
	}

	if buildNumber < arcconfMinimalBuildVersion {
		level.Error(logger).Log("msg", "exec bin version is too low", "buildNumber", buildNumber)
		return nil, fmt.Errorf("exec bin version is too low")
	}

	return &Exporter{
		rw:                       &sync.RWMutex{},
		executablePath:           executablePath,
		minimalInterval:          miniExecInterval,
		CmdErr:                   make(map[string]error),
		CmdStderr:                make(map[string][]byte),
		CmdStdout:                make(map[string][]byte),
		CommandLastedExecuteTime: make(map[string]time.Time),
		skipCollectSMARTForJDOB:  skipCollectSMARTForJDOB,
		collectSMART:             enableSMART,
		logger:                   logger,
		controllerCount:          controllerCount,
	}, nil
}

func (e *Exporter) execCmd(args ...string) (stdout, stderr []byte, err error) {
	e.rw.Lock()
	defer e.rw.Unlock()

	ak := strings.Join(args, "+")
	if lastExecTime, ok := e.CommandLastedExecuteTime[ak]; ok {
		if time.Since(lastExecTime) < e.minimalInterval {
			level.Debug(e.logger).Log("msg", "command executed too frequently", "args", args, "lastExecTime", lastExecTime, "interval", time.Since(lastExecTime))
			return e.CmdStdout[ak], e.CmdStderr[ak], e.CmdErr[ak]
		}
	}

	cmd := exec.Command(e.executablePath, args...)
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	err = cmd.Run()

	if err != nil {
		level.Error(e.logger).Log("msg", "command run failed", "args", args, "err", err)
	}
	e.CmdErr[ak] = err
	e.CmdStdout[ak] = stdoutBuf.Bytes()
	e.CmdStderr[ak] = stderrBuf.Bytes()
	e.CommandLastedExecuteTime[ak] = time.Now()

	level.Debug(e.logger).Log("msg", "command executed", "args", args, "stdout", stdoutBuf.String(), "stderr", stderrBuf.String())

	return stdoutBuf.Bytes(), stderrBuf.Bytes(), err
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- ctrlInfo
	ch <- ctrlHealthy
	ch <- ctrlStatus
	ch <- ctrlTemperature

	ch <- bocHealthy
	ch <- bocCount

	ch <- ldInfo
	ch <- ldHealthy
	ch <- ldStatus

	ch <- pdInfo
	ch <- pdHealthy
	ch <- pdStatus
	ch <- pdSmartHealthy
	ch <- pdSmartRawValue
	ch <- pdSmartCurrentValue

}

func (e *Exporter) Collect(metrics chan<- prometheus.Metric) {
	for controllerId := 1; controllerId <= e.controllerCount; controllerId++ {
		e.collectEveryController(controllerId, metrics)
	}
}

func (e *Exporter) collectEveryController(controllerId int, metrics chan<- prometheus.Metric) {
	ctrlOutput, _, err := e.execCmd("GETCONFIGJSON", strconv.Itoa(controllerId))
	if err != nil {
		level.Error(e.logger).Log("msg", "GETCONFIGJSON failed", "controllerId", controllerId, "err", err)
		return
	}
	var smartOutput []byte
	if e.collectSMART {
		smartOutput, _, err = e.execCmd("GETSMARTSTATS", strconv.Itoa(controllerId))
		if err != nil {
			level.Error(e.logger).Log("msg", "GETSMARTSTATS failed", "controllerId", controllerId, "err", err)
			return
		}
	}
	e.parserData(ctrlOutput, smartOutput, metrics)
}
func (e *Exporter) parserData(stdout, smart []byte, metrics chan<- prometheus.Metric) {
	// GETCONFIGJSON will have line, first line is Controller Count
	// We will use gjson to get info from the second line

	stdoutArray := bytes.Split(stdout, []byte{'\n'})
	if len(stdoutArray) < 2 {
		level.Error(e.logger).Log("msg", "GETCONFIGJSON stdout line too less")
		return
	}

	ctrlJson := gjson.Get(string(stdoutArray[1]), "Controller")
	ctrlId := ctrlJson.Get("controllerID").String()
	metrics <- prometheus.MustNewConstMetric(ctrlInfo, prometheus.GaugeValue, 1,
		ctrlId,
		ctrlJson.Get("firmwareVersion").String(),
		ctrlJson.Get("deviceName").String(),
		ctrlJson.Get("serialNumber").String())

	ctrlStatusCode := ctrlJson.Get("controllerState").Int()
	if ctrlStatusCode == 0 {
		metrics <- prometheus.MustNewConstMetric(ctrlHealthy, prometheus.GaugeValue, 1, ctrlId)
	} else {
		metrics <- prometheus.MustNewConstMetric(ctrlHealthy, prometheus.GaugeValue, 0, ctrlId)
	}
	metrics <- prometheus.MustNewConstMetric(ctrlStatus, prometheus.GaugeValue, float64(ctrlStatusCode), ctrlId)

	// Temperature
	for _, temp := range ctrlJson.Get("TemperatureSensor").Array() {
		metrics <- prometheus.MustNewConstMetric(
			ctrlTemperature, prometheus.GaugeValue, temp.Get("currentValue").Float(),
			ctrlId, temperatureSensorLocation(int(temp.Get("temperatureSensorLocation").Int())))
	}

	// batteryOrCapacitor
	metrics <- prometheus.MustNewConstMetric(bocCount, prometheus.GaugeValue,
		ctrlJson.Get("batteryOrCapacitorCount").Float(), ctrlId)

	if strings.ToLower(ctrlJson.Get("batteryOrCapacitorStatus").String()) == "ok" {
		metrics <- prometheus.MustNewConstMetric(bocHealthy, prometheus.GaugeValue, 1, ctrlId)
	} else {
		metrics <- prometheus.MustNewConstMetric(bocHealthy, prometheus.GaugeValue, 0, ctrlId)
	}

	for _, vd := range ctrlJson.Get("LogicalDrive").Array() {
		metrics <- prometheus.MustNewConstMetric(ldInfo, prometheus.GaugeValue, 1,
			ctrlId, vd.Get("logicalDriveID").String(),
			raidLevel(int(vd.Get("raidLevel").Int())), vd.Get("size").String())
		if vd.Get("status").String() == "2" {
			metrics <- prometheus.MustNewConstMetric(ldHealthy, prometheus.GaugeValue, 1,
				ctrlId, vd.Get("logicalDriveID").String())
		} else {
			metrics <- prometheus.MustNewConstMetric(ldHealthy, prometheus.GaugeValue, 0,
				ctrlId, vd.Get("logicalDriveID").String())
		}
		metrics <- prometheus.MustNewConstMetric(ldStatus, prometheus.GaugeValue, vd.Get("status").Float(),
			ctrlId, vd.Get("logicalDriveID").String())
	}

	var diskIdIsJDOB = make(map[int]bool)

	for _, channel := range ctrlJson.Get("Channel").Array() {
		if !channel.Get("HardDrive").IsArray() {
			continue
		}
		for _, drive := range channel.Get("HardDrive").Array() {
			driveId := drive.Get("deviceID").String()
			metrics <- prometheus.MustNewConstMetric(pdInfo, prometheus.GaugeValue, 1,
				ctrlId, driveId,
				drive.Get("firmwareLevel").String(),
				drive.Get("model").String(),
				drive.Get("serialNumber").String(),
				drive.Get("enclosureID").String(),
				drive.Get("slotID").String(),
				deviceType(int(drive.Get("deviceType").Int())),
			)

			state := drive.Get("state").Int()
			if state == 0 || state == 1 || state == 3 {
				// 0 ready, 1 online, 3 standby
				metrics <- prometheus.MustNewConstMetric(pdHealthy, prometheus.GaugeValue, 1, ctrlId, driveId)
			} else {
				metrics <- prometheus.MustNewConstMetric(pdHealthy, prometheus.GaugeValue, 0, ctrlId, driveId)
			}
			metrics <- prometheus.MustNewConstMetric(pdStatus, prometheus.GaugeValue, float64(state), ctrlId, driveId)
			diskIdIsJDOB[int(drive.Get("id").Int())] = drive.Get("jbod").Bool()
		}

	}
	// GETSMARTSTATUS
	if !e.collectSMART {
		return
	}

	var ataSmartStatus, sasSmartStatus SmartStats
	smartSplitArray := bytes.Split(smart, []byte{'\n', '\n'})
	if len(smartSplitArray) < 5 {
		level.Error(e.logger).Log("msg", "GETSMARTSTATS stdout after split line too less")
		return
	}

	if err := xml.Unmarshal(smartSplitArray[2], &ataSmartStatus); err != nil {
		level.Error(e.logger).Log("msg", "GETSMARTSTATS unmarshal ataSmartStatus failed", "err", err)
		return
	}

	if err := xml.Unmarshal(smartSplitArray[4], &sasSmartStatus); err != nil {
		level.Error(e.logger).Log("msg", "GETSMARTSTATS unmarshal sasSmartStatus failed", "err", err)
		return
	}

	for _, phySmartStats := range ataSmartStatus.PhySmartStats {
		if diskIdIsJDOB[phySmartStats.Id] && e.skipCollectSMARTForJDOB {
			continue
		}
		for _, attr := range phySmartStats.Attribute {
			metrics <- prometheus.MustNewConstMetric(
				pdSmartCurrentValue, prometheus.GaugeValue, float64(attr.NormalizedCurrent),
				ctrlId, strconv.Itoa(phySmartStats.Id), attr.Id, attr.Name)
			metrics <- prometheus.MustNewConstMetric(
				pdSmartRawValue, prometheus.GaugeValue, float64(attr.RawValue),
				ctrlId, strconv.Itoa(phySmartStats.Id), attr.Id, attr.Name)
			if attr.Status == "OK" {
				metrics <- prometheus.MustNewConstMetric(
					pdSmartHealthy, prometheus.GaugeValue, 1,
					ctrlId, strconv.Itoa(phySmartStats.Id), attr.Id, attr.Name)
			} else {
				metrics <- prometheus.MustNewConstMetric(
					pdSmartHealthy, prometheus.GaugeValue, 0,
					ctrlId, strconv.Itoa(phySmartStats.Id), attr.Id, attr.Name)
			}
		}
	}
	for _, phySmartStats := range sasSmartStatus.PhySmartStats {
		if diskIdIsJDOB[phySmartStats.Id] && e.skipCollectSMARTForJDOB {
			continue
		}
		for _, attr := range phySmartStats.Attribute {
			val, err := strconv.ParseFloat(attr.Value, 64)
			if err == nil {
				metrics <- prometheus.MustNewConstMetric(
					pdSmartCurrentValue, prometheus.GaugeValue, val,
					ctrlId, strconv.Itoa(phySmartStats.Id), attr.Id, attr.Name)
			} else {
				v := strings.ToLower(attr.Value)
				if v == "passed" || v == "ok" {
					metrics <- prometheus.MustNewConstMetric(
						pdSmartHealthy, prometheus.GaugeValue, 1,
						ctrlId, strconv.Itoa(phySmartStats.Id), attr.Id, attr.Name)
				} else {
					metrics <- prometheus.MustNewConstMetric(
						pdSmartHealthy, prometheus.GaugeValue, 0,
						ctrlId, strconv.Itoa(phySmartStats.Id), attr.Id, attr.Name)
				}
			}
		}

	}
}

func main() {
	var (
		webConfig               = webflag.AddFlags(kingpin.CommandLine, ":10101")
		metricsPath             = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
		execBin                 = kingpin.Flag("adaptec.command", "Path to the arcconf binary.").Default("arcconf").String()
		minimalInterval         = kingpin.Flag("adaptec.minimal-execute-interval", "Minimal execute interval for the exporter.").Default("60s").Duration()
		extraLookupPaths        = kingpin.Flag("adaptec.extra-lookup-paths", "Extra lookup paths for the exporter.").Default("").String()
		collectSMART            = kingpin.Flag("adaptec.collect-smart-metric", "Enable SMART metrics").Default("true").Bool()
		skipCollectSMARTForJDOB = kingpin.Flag("adaptec.skip-collect-smart-for-jbod", "Skip collect SMART metrics for JBOD").Default("false").Bool()
		writeFilepath           = kingpin.Flag("write-filepath", "Write metrics to file").Default("").String()
	)

	logConfig := &promlog.Config{}
	flag.AddFlags(kingpin.CommandLine, logConfig)
	kingpin.Version(version.Print("adaptec_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()
	logger := promlog.New(logConfig)

	level.Info(logger).Log(version.Info())
	level.Info(logger).Log(version.BuildContext())

	exporter, err := NewExporter(*execBin, append(strings.Split(*extraLookupPaths, string(os.PathListSeparator)),
		strings.Split(os.Getenv("PATH"), string(os.PathListSeparator))...), *minimalInterval,
		*collectSMART, *skipCollectSMARTForJDOB, logger)

	if err != nil {
		os.Exit(1)
	}

	prometheus.MustRegister(exporter)
	prometheus.MustRegister(collector_version.NewCollector("adaptec_exporter"))

	if *writeFilepath != "" {
		err = prometheus.WriteToTextfile(*writeFilepath, prometheus.DefaultGatherer)
		if err != nil {
			level.Error(logger).Log("msg", "Error writing to file", "err", err)
			os.Exit(0)
		} else {
			level.Info(logger).Log("msg", "Metrics written to file", "file", *writeFilepath)
			os.Exit(0)
		}
	}

	http.Handle(*metricsPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html>
             <head><title>Adaptec Exporter</title></head>
             <body>
             <h1>Adaptec Exporter</h1>
             <p><a href='` + *metricsPath + `'>Metrics</a></p>
             </body>
             </html>`))
	})
	srv := &http.Server{}
	if err := web.ListenAndServe(srv, webConfig, logger); err != nil {
		level.Error(logger).Log("msg", "Error starting HTTP server", "err", err)
		os.Exit(1)
	}
}
