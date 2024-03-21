package main

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/cloudflare/cloudflare-go"
)

var logger *log.Logger
var ClouflareAPiClient *cloudflare.API
var CloudflareaApiCallsCount int

func main() {

	homeDir, _ := os.UserHomeDir()
	logfilePath := filepath.Join(homeDir, "cfbulkdeletion.log")

	if len(os.Args) < 4 || len(os.Args) > 4 {
		log.Fatal("provide domain record file path, cloudflare token and zone Id")
	}
	// else if len(os.Args) == 4 {
	// 	logfilePath = os.Args[3]
	// 	if !strings.HasSuffix(logfilePath, ".log") {
	// 		log.Fatal("wrong file path format: please provide log file path like /path/to/filename.log")
	// 	}
	// }

	initLogger(logfilePath)
	logger.Println("cloudflare bulk dns record deletion")
	dnsRecordFilePath := os.Args[1]
	dnsList := getDNSList(dnsRecordFilePath)
	cloudflareApiToken := os.Args[2]
	zoneId := os.Args[3]
	//cloudflareApiToken:"t-8InF-H5wjS066GxjXMVKUJXxs3WFvoLAK_pav0"
	var err error
	ClouflareAPiClient, err = cloudflare.NewWithAPIToken(cloudflareApiToken)
	if err != nil {
		logger.Fatal(err)
	}
	err = bulkDnsRecordRemoval(dnsList, zoneId)
	if err != nil {
		logger.Fatal(err)
	}

}

func initLogger(logfilePath string) {
	file, err := os.OpenFile(logfilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	multiWriter := io.MultiWriter(file, os.Stdout)
	logger = log.New(multiWriter, "cf-bulk-deletion ", log.LstdFlags)
}

func isEmptyLine(line string) bool {
	line = strings.TrimSpace(line)
	return line == "" //delete empty lines
}

func getDNSList(dnsRecordFilePath string) []string {
	dnsRecord, err := os.ReadFile(dnsRecordFilePath)
	if err != nil {
		log.Fatal(err)
	}

	content := string(dnsRecord)

	lines := strings.Split(content, "\n")
	// Use the filter function from the slices package to remove empty lines
	dnsRecords := slices.DeleteFunc(lines, isEmptyLine)
	return dnsRecords
}

func bulkDnsRecordRemoval(dnsList []string, zoneID string) error {
	if !isExceededRateLimit() {
		recs, _, err := ClouflareAPiClient.ListDNSRecords(context.Background(), cloudflare.ZoneIdentifier(zoneID), cloudflare.ListDNSRecordsParams{})
		if err != nil {
			logger.Println(err)
			return err
		}

		for _, rec := range recs {
			if strings.HasPrefix(rec.Name, "_acme") {
				deleteDNSrecord(zoneID, rec)
				logger.Print("deleting dns record Id:" + rec.ID)
			} else {
				logger.Print("not starts with _acme avalable in zone" + rec.Name)
			}
		}

		// for _, dns := range dnsList {
		// 	var dnsRecord cloudflare.DNSRecord
		// 	if slices.ContainsFunc(recs, func(rec cloudflare.DNSRecord) bool {
		// 		dnsRecord = rec
		// 		return rec.Name == dns
		// 	}) {
		// 		deleteDNSrecord(zoneID, dnsRecord)
		// 		logger.Print("deleting dns record Id:" + dnsRecord.ID)
		// 	} else {
		// 		logger.Print(dns + " not avalable in zone" + zoneID)
		// 	}
		// }
	}

	return nil
}

func deleteDNSrecord(zoneID string, rec cloudflare.DNSRecord) error {
	if !isExceededRateLimit() {
		err := ClouflareAPiClient.DeleteDNSRecord(context.Background(), cloudflare.ZoneIdentifier(zoneID), rec.ID)
		if err != nil {
			log.Print(err)
			return err
		}
		CloudflareaApiCallsCount++
	}
	return nil
}

func isExceededRateLimit() bool {
	if CloudflareaApiCallsCount >= 600 {
		logger.Print("Exeeds cloudflare rate limit:", CloudflareaApiCallsCount)
		time.Sleep(6 * time.Minute)
		return true
	} else {
		return false
	}
}
