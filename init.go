package sham

import log "github.com/sirupsen/logrus"

func init() {
	// Setup logrus
	//log.SetReportCaller(true)
	log.SetFormatter(&log.TextFormatter{
		ForceColors: true,
	})
	log.SetLevel(log.TraceLevel)
}
