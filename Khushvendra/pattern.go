package main

import(
	"fmt"
)

func main2() {
// 1.
// *
// **
// ***
// ****
// *****

	for i:=1; i<6; i++ {
		for j:=1; j<=i; j++ {
			fmt.Print("*")
		}
		fmt.Println()
	}
	fmt.Println()

// 2.
//     *
//    **
//   ***
//  ****
// *****

	for i:=1; i<6; i++ {
		for k:=1; k<=5-i; k++ {
			fmt.Print(" ")
		}
		for j:=1; j<=i; j++ {
			fmt.Print("*")
		}
		fmt.Println()
	} 
	fmt.Println()

// 3.
//    *
//   ***
//  *****
// *******
		for i:=1; i<=4; i++ {
		for k:=1; k<=4-i; k++ {
			fmt.Print(" ")
		}
		for j:=1; j<=i+(i-1); j++ {
			fmt.Print("*")
		}
		fmt.Println()
	} 
	fmt.Println()

// 4.)
// 1
// 2 3
// 4 5 6
// 7 8 9 10
		var l int = 1
		for i:=1; i<5; i++ {
		for j:=1; j<=i; j++ {
			fmt.Print(l)
			fmt.Print(" ")
			l++
		}
		fmt.Println()
	}
	
}