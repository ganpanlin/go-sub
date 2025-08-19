package main

import (
	"fmt"
	"go-sub/internal/parser"
	"io"
	"net/http"
	"time"
)

func main() {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get("https://checkhere.top/link/ZwxWBRNeNtdo4gzq?clash=1")
	if err != nil {
		fmt.Println("fetch err:", err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Body: %d bytes\n", len(body))

	config, err := parser.ParseYAML(body)
	if err != nil {
		fmt.Println("parse err:", err)
		return
	}
	fmt.Println("Keys:")
	for k, v := range config {
		fmt.Printf("  %s: %T\n", k, v)
	}
	if proxies, ok := config["proxies"].([]interface{}); ok {
		fmt.Printf("Proxies: %d\n", len(proxies))
		for i, p := range proxies {
			if i >= 3 {
				break
			}
			fmt.Printf("  [%d] %v\n", i, p)
		}
	} else {
		fmt.Printf("proxies type: %T = %v\n", config["proxies"], config["proxies"])
	}
}
