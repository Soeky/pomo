package web

import (
	"fmt"
	"net/http"
	"time"
)

func HealthCheck(baseURL string) error {
	client := &http.Client{Timeout: 2 * time.Second}
	res, err := client.Get(baseURL + "/healthz")
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("health status: %d", res.StatusCode)
	}
	return nil
}
