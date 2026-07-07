package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// fetch_binance_klines fetches 500 5m BTCUSDT candles from Binance.
// Returns closing prices as []float64 in chronological order.
func fetch_binance_klines() []float64 {
	url := "https://api.binance.com/api/v3/klines?symbol=BTCUSDT&interval=5m&limit=500"
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		panic("Binance HTTP error: " + err.Error())
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		panic("Binance read error: " + err.Error())
	}

	// Binance klines: [][open, high, low, close, volume, ...]
	// Each kline is []interface{} with price fields as strings
	var raw [][]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		panic("Binance JSON error: " + err.Error())
	}

	prices := make([]float64, 0, len(raw))
	for _, k := range raw {
		if len(k) < 5 {
			continue
		}
		closeStr, ok := k[4].(string)
		if !ok {
			continue
		}
		price, err := strconv.ParseFloat(closeStr, 64)
		if err != nil {
			continue
		}
		prices = append(prices, price)
	}
	return prices
}

// fetch_binance_volumes fetches 500 5m BTCUSDT volumes.
func fetch_binance_volumes() []float64 {
	url := "https://api.binance.com/api/v3/klines?symbol=BTCUSDT&interval=5m&limit=500"
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		panic("Binance HTTP error: " + err.Error())
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var raw [][]interface{}
	json.Unmarshal(body, &raw)

	vols := make([]float64, 0, len(raw))
	for _, k := range raw {
		if len(k) < 6 {
			continue
		}
		volStr, ok := k[5].(string)
		if !ok {
			continue
		}
		vol, err := strconv.ParseFloat(volStr, 64)
		if err != nil {
			continue
		}
		vols = append(vols, vol)
	}
	return vols
}

// verbose_binance_kline provides a formatted string of the most recent kline.
func verbose_binance_kline() string {
	url := "https://api.binance.com/api/v3/klines?symbol=BTCUSDT&interval=5m&limit=1"
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "N/A"
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var raw [][]interface{}
	json.Unmarshal(body, &raw)
	if len(raw) == 0 || len(raw[0]) < 12 {
		return "N/A"
	}
	k := raw[0]
	openT := int64(k[0].(float64))
	closeStr, _ := k[4].(string)
	highStr, _ := k[2].(string)
	lowStr, _ := k[3].(string)
	volStr, _ := k[5].(string)

	t := time.UnixMilli(openT)
	return fmt.Sprintf("%s  O:%s H:%s L:%s C:%s V:%s",
		t.Format("01-02 15:04"), k[1], highStr, lowStr, closeStr, volStr)
}
