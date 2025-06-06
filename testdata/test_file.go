package testfile

import "fmt"

// This is a test function
func testFunction() {
	fmt.Println("Hello, World!")
}

// Another function with TODO comment
func anotherFunction() {
	// TODO: implement this function
	fmt.Println("Another function")
}

// Function with error handling
func functionWithError() error {
	return fmt.Errorf("this is an error")
}

// Example function
func exampleMain() {
	testFunction()
	anotherFunction()

	if err := functionWithError(); err != nil {
		fmt.Printf("Error: %v\n", err)
	}
}
