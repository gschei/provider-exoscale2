package exoapi

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/exoscale/egoscale/v2/api"
)

const (
	apiURL = "https://api-ZONE.exoscale.com/v2/ENDPOINT"
)

type SksClusters struct {
	Clusters []SksCluster `json:"sks-clusters"`
}

type SksCluster struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Cni         string `json:"cni,omitempty"`
	Level       string `json:"level,omitempty"`
}

func getUrl(zone string, endpoint string) string {
	return strings.Replace(strings.Replace(apiURL, "ZONE", zone, 1), "ENDPOINT", endpoint, 1)
}

func RetrieveClusters(exoApiKey string, exoApiSecret string, zone string) (*SksClusters, error) {

	provider, err := api.NewSecurityProvider(exoApiKey, exoApiSecret)
	if err != nil {
		return nil, err
	}

	req, _ := http.NewRequest("GET", getUrl(zone, "sks-cluster"), nil)
	provider.Intercept(nil, req)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	//fmt.Printf("client: response body: %s\n", resBody)

	var clusters SksClusters
	err = json.Unmarshal(resBody, &clusters)
	if err != nil {
		return nil, err
	}

	//fmt.Printf("parsed json: %v", clusters)

	return &clusters, nil
}
