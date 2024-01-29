package main

import (
	"context"
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

	if len(os.Args) < 3 || len(os.Args) > 4 {
		log.Fatal("provide domain record file path and cloudflare token")
	} else if len(os.Args) == 4 {
		logfilePath = os.Args[3]
		if !strings.HasSuffix(logfilePath, ".log") {
			log.Fatal("wrong file path format: please provide log file path like /path/to/filename.log")
		}
	}

	logger = getLogger(logfilePath)
	logger.Println("cloudflare bulk dns record deletion")
	dnsRecordFilePath := os.Args[1]
	domains := getDomains(dnsRecordFilePath)
	cloudflareApiToken := os.Args[2]
	//cloudflareApiToken:"t-8InF-H5wjS066GxjXMVKUJXxs3WFvoLAK_pav0"
	var err error
	ClouflareAPiClient, err = cloudflare.NewWithAPIToken(cloudflareApiToken)
	if err != nil {
		logger.Fatal(err)
	}
	err = bulkDnsRecordRemoval(domains)
	if err != nil {
		logger.Fatal(err)
	}

}

func getLogger(logfilePath string) *log.Logger {
	file, err := os.OpenFile(logfilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	logger := log.New(file, "cf-bulk-deletion", log.LstdFlags)
	return logger
}

func isEmptyLine(line string) bool {
	line = strings.TrimSpace(line)
	return line == "" //delete empty lines
}

func getDomains(dnsRecordFilePath string) []string {
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

func bulkDnsRecordRemoval(domains []string) error {
	ctx := context.Background()

	for _, domain := range domains {
		if !isExceededRateLimit() {

			zoneID, err := ClouflareAPiClient.ZoneIDByName(domain)
			if err != nil {
				log.Print(err)
				return err
			}
			if !isExceededRateLimit() {
				recs, _, err := ClouflareAPiClient.ListDNSRecords(ctx, cloudflare.ZoneIdentifier(zoneID), cloudflare.ListDNSRecordsParams{})
				if err != nil {
					log.Fatal(err)
					return err
				}
				CloudflareaApiCallsCount++
				err = deleteDNSrecord(zoneID, recs)
				if err != nil {
					log.Print(err)
					return err
				}
			}
		}
	}
	return nil
}

func deleteDNSrecord(zoneID string, recs []cloudflare.DNSRecord) error {
	ctx := context.Background()
	for _, rec := range recs {
		if strings.HasPrefix(rec.Name, "_acme-challenge") {
			if !isExceededRateLimit() {
				err := ClouflareAPiClient.DeleteDNSRecord(ctx, cloudflare.ZoneIdentifier(zoneID), rec.ID)
				if err != nil {
					log.Print(err)
					return err
				}
				CloudflareaApiCallsCount++
			}

		}
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
