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

func temperatureSensorLocation(raw int) string {
	switch raw {
	case 0:
		return "Inlet Ambient"
	case 1:
		return "ASIC"
	case 2:
		return "Top"
	case 3:
		return "Bottom"
	case 4:
		return "Front"
	case 5:
		return "Back"
	case 6:
		return "Cache"
	case 7:
		return "Capacitor"
	case 8:
		return "Unknown"
	}
	return "<undefined>"
}

func raidLevel(raw int) string {
	switch raw {
	case 0:
		return "0"
	case 1:
		return "1"
	case 2:
		return "NA"
	case 3:
		return "NA"
	case 4:
		return "NA"
	case 5:
		return "5"
	case 6:
		return "NA"
	case 7:
		return "NA"
	case 8:
		return "NA"
	case 9:
		return "10"
	case 10:
		return "NA"
	case 11:
		return "50"
	case 12:
		return "NA"
	case 13:
		return "NA"
	case 14:
		return "NA"
	case 15:
		return "NA"
	case 16:
		return "6"
	case 17:
		return "60"
	case 18:
		return "NA"
	case 19:
		return "NA"
	case 20:
		return "1 Triple"
	case 21:
		return "10 Triple"
	case 22:
		return "NA"
	case 23:
		return "NA"
	case 24:
		return "RAIDMAX"
	case 0x7FFFFFF:
		return "Unknown"
	}
	return "<undefined>"
}

func deviceType(raw int) string {
	switch raw {
	case 0:
		return "Hard drive"
	case 1:
		return "Tape drive"
	case 2:
		return "Printer"
	case 3:
		return "Enclosure"
	case 4:
		return "Worm device"
	case 5:
		return "CD ROM"
	case 6:
		return "Scanner"
	case 7:
		return "Optical device"
	case 8:
		return "Changer"
	case 9:
		return "Comm device"
	case 10:
		return "Unknown"
	case 12:
		return "Storage array controller"
	case 13:
		return "Enclosure services"
	case 14:
		return "Simplified direct access"
	case 15:
		return "Optical card"
	}
	return "<undefined>"
}
