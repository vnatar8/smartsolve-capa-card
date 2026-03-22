package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
)

func downloadCAPADetail(token string, capaNumber string) (string, error) {
	reqURL := fmt.Sprintf(
		"%sV2SmartReport.aspx?RptName=CAPA_DETAIL&RECORD_NUMBER=%s&RECORD_REVISION=&Token=%s",
		config.APIURL, url.QueryEscape(capaNumber), url.QueryEscape(token),
	)

	resp, err := http.Get(reqURL)
	if err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	if len(data) < 1024 || string(data[:5]) != "%PDF-" {
		return "", fmt.Errorf("invalid PDF response (%d bytes)", len(data))
	}

	tmpFile, err := os.CreateTemp("", "capa-detail-*.pdf")
	if err != nil {
		return "", err
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return "", err
	}
	tmpFile.Close()

	fmt.Printf("Downloaded %s (%d bytes) to %s\n", capaNumber, len(data), filepath.Base(tmpPath))
	return tmpPath, nil
}
