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

import "github.com/prometheus/client_golang/prometheus"

var (
	ctrlInfo = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "controller", "controller_info"),
		"Adaptec Controller info",
		[]string{"controller", "fwversion", "model", "serial"}, nil,
	)

	ctrlHealthy = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "controller", "healthy"),
		"Adaptec Controller status is healthy",
		[]string{"controller"}, nil,
	)
	ctrlStatus = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "controller", "status"),
		"Adaptec Controller raw status code (ask vendor to get detail info)",
		[]string{"controller"}, nil,
	)

	ctrlTemperature = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "controller", "temperature"),
		"Adaptec Controller temperature in Celsius",
		[]string{"controller", "location"}, nil,
	)

	// bbu or super capacitor
	bocHealthy = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "battery_or_capacitor", "healthy"),
		"Adaptec Controller BBU status is Ok",
		[]string{"controller"}, nil)
	bocCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "battery_or_capacitor", "count"),
		"Adaptec Controller BBU count",
		[]string{"controller"}, nil)

	// vd info
	ldInfo = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "ld", "info"),
		"Logical Drive info under controller",
		[]string{"controller", "ld", "raid_level", "size"}, nil)
	// check scheme, 2 is ok
	ldHealthy = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "ld", "healthy"),
		"Logical Drive status is healthy",
		[]string{"controller", "ld"}, nil)
	ldStatus = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "ld", "status"),
		"Logical Drive raw status code (ask vendor to get detail info)",
		[]string{"controller", "ld"}, nil)

	// Disk info
	pdInfo = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "pd", "info"),
		"Physical Drive info under controller",
		[]string{"controller", "pd", "fwversion", "model", "serial", "enclosure_id", "slot_id", "type"}, nil)
	pdHealthy = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "pd", "healthy"),
		"Physical Drive status is healthy",
		[]string{"controller", "pd"}, nil)
	pdStatus = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "pd", "status"),
		"Physical Drive raw status code (ask vendor to get detail info)",
		[]string{"controller", "pd"}, nil)

	// not nvme test
	// sas disk will have not smart_id
	pdSmartRawValue = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "pd_smart", "raw"),
		"Disk smart raw value",
		[]string{"controller", "pd", "smart_id", "name"}, nil)

	pdSmartCurrentValue = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "pd_smart", "current"),
		"Disk smart current value",
		[]string{"controller", "pd", "smart_id", "name"}, nil)

	pdSmartHealthy = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "pd_smart", "status"),
		"Disk smart current value",
		[]string{"controller", "pd", "smart_id", "name"}, nil)
)
