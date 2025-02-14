package verify

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

type EtherscanResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Result  string `json:"result"`
}

func getAPIEndpoint(chainID uint64) string {
	switch chainID {
	case 1:
		return "https://api.etherscan.io/api" // mainnet
	case 11155111:
		return "https://api-sepolia.etherscan.io/api" // sepolia
	default:
		return ""
	}
}

func (v *Verifier) verifyContract(address common.Address, contractName string) error {
	verified, err := v.isVerified(address)
	if err != nil {
		return fmt.Errorf("failed to check verification status: %w", err)
	}
	if verified {
		v.log.Info("Contract is already verified", "name", contractName, "address", address.Hex())
		v.numSkipped++
		return nil
	}

	v.log.Info("Formatting etherscan verification request", "name", contractName, "address", address.Hex())
	source, err := v.getContractArtifact(contractName)
	if err != nil {
		return fmt.Errorf("failed to get contract source: %w", err)
	}

	optimized := "0"
	if source.OptimizationUsed {
		optimized = "1"
	}

	data := url.Values{
		"apikey":                {v.apiKey},
		"module":                {"contract"},
		"action":                {"verifysourcecode"},
		"contractaddress":       {address.Hex()},
		"codeformat":            {"solidity-standard-json-input"},
		"sourceCode":            {source.StandardInput},
		"contractname":          {source.ContractName},
		"compilerversion":       {fmt.Sprintf("v%s", source.CompilerVersion)},
		"optimizationUsed":      {optimized},
		"runs":                  {fmt.Sprintf("%d", source.OptimizationRuns)},
		"evmversion":            {source.EVMVersion},
		"constructorArguements": {source.ConstructorArgs},
	}

	req, err := http.NewRequest("POST", v.etherscanUrl, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create verification request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := v.sendRateLimitedRequest(req)
	if err != nil {
		return fmt.Errorf("failed to submit verification request: %w", err)
	}
	defer resp.Body.Close()

	var result EtherscanResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Status != "1" {
		return fmt.Errorf("verification request failed: status=%s message=%s result=%s",
			result.Status, result.Message, result.Result)
	}
	v.log.Info("Verification request submitted", "name", contractName, "address", address.Hex())
	err = v.checkVerificationStatus(result.Result)
	if err == nil {
		v.log.Info("Verification complete", "name", contractName, "address", address.Hex())
		v.numVerified++
	}
	return err
}

// sendRateLimitedRequest is a helper function which waits for a rate limit token
// before sending a request
func (v *Verifier) sendRateLimitedRequest(req *http.Request) (*http.Response, error) {
	if err := v.rateLimiter.Wait(context.Background()); err != nil {
		return nil, fmt.Errorf("rate limiter error: %w", err)
	}

	return http.DefaultClient.Do(req)
}

func (v *Verifier) isVerified(address common.Address) (bool, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s?module=contract&action=getabi&address=%s&apikey=%s",
		v.etherscanUrl, address.Hex(), v.apiKey), nil)
	if err != nil {
		return false, err
	}

	resp, err := v.sendRateLimitedRequest(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	var result EtherscanResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, err
	}

	v.log.Debug("Contract verification status", "status", result.Status, "message", result.Message)
	return result.Status == "1", nil
}

func (v *Verifier) checkVerificationStatus(reqId string) error {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s?apikey=%s&module=contract&action=checkverifystatus&guid=%s",
		v.etherscanUrl, v.apiKey, reqId), nil)
	if err != nil {
		return fmt.Errorf("failed to create checkverifystatus request: %w", err)
	}

	for i := 0; i < 10; i++ { // Try 10 times with increasing delays
		v.log.Info("Checking verification status", "guid", reqId)
		time.Sleep(time.Duration(i+2) * time.Second)

		resp, err := v.sendRateLimitedRequest(req)
		if err != nil {
			return fmt.Errorf("failed to send checkverifystatus request: %w", err)
		}
		defer resp.Body.Close()

		var result struct {
			Status  string `json:"status"`
			Message string `json:"message"`
			Result  string `json:"result"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return fmt.Errorf("failed to decode checkverifystatus response: %w", err)
		}

		if result.Status == "1" {
			return nil
		}
		if result.Result == "Already Verified" {
			v.log.Info("Contract is already verified")
			return nil
		}
		if result.Result != "Pending in queue" {
			return fmt.Errorf("verification failed: %s, %s", result.Result, result.Message)
		}
	}
	return fmt.Errorf("verification timed out")
}
