package utils

import (
	"fmt"
	"log"
)

func ExampleIsValidURL() {
	// Valid URLs
	fmt.Println(IsValidURL("https://example.com"))
	fmt.Println(IsValidURL("http://sub.example.com/path?query=param"))
	fmt.Println(IsValidURL("http://localhost:8080"))

	// Invalid URLs
	fmt.Println(IsValidURL("examples_test.com"))          // Missing scheme
	fmt.Println(IsValidURL("ftp://files.com"))            // Invalid scheme
	fmt.Println(IsValidURL("https://"))                   // Missing host
	fmt.Println(IsValidURL("https:// examples_test.com")) // Contains space

	// Output:
	// true
	// true
	// true
	// false
	// false
	// false
	// false
}

func ExampleGenerateRandomKey() {
	// Generate a random key
	key := GenerateRandomKey()

	// The key will always be 8 characters long
	fmt.Println(len(key) == 8)

	// The key consists only of alphanumeric characters
	isAlphanumeric := true
	for _, c := range key {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
			isAlphanumeric = false
			break
		}
	}
	fmt.Println(isAlphanumeric)

	// Note: The actual output varies since it's random
	// Output:
	// true
	// true
}

func ExampleGenerateRandomKey_usage() {
	// Typical usage for generating API keys or tokens
	apiKey := GenerateRandomKey()

	// Use the generated key in your application
	log.Printf("Generated API key: %s", apiKey)

	// You can generate multiple unique keys
	key1 := GenerateRandomKey()
	key2 := GenerateRandomKey()

	// They will (almost certainly) be different
	fmt.Println(key1 != key2)

	// Output:
	// true
}

func ExampleIsValidURL_usage() {
	urls := []string{
		"https://api.example.com/v1/users",
		"http://localhost:3000",
		"invalid url",
		"ssh://server.com",
	}

	for _, u := range urls {
		if IsValidURL(u) {
			fmt.Printf("Valid URL: %s\n", u)
		} else {
			fmt.Printf("Invalid URL: %s\n", u)
		}
	}

	// Output:
	// Valid URL: https://api.example.com/v1/users
	// Valid URL: http://localhost:3000
	// Invalid URL: invalid url
	// Invalid URL: ssh://server.com
}
