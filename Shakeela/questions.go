package main
import "fmt"
func odd_even(a int) {
	if a%2==0{
		fmt.Println("even")
	} else {
		fmt.Println("odd")
	}
}
func largest(arr [3]int ) int {
	var s int = arr[0]
	for i:= 1; i<3; i++{
		if arr[i]> s {
			s= arr[i]
		}
	}
	return s
}
func main() {
	var no int
	fmt.Println("Enter a number ")
	fmt.Scan(&no)
	odd_even(no)
	fmt.Println("Sum of first N numbers:", (no*(no+1)/2) )
	fmt.Println("Suggest 3 numbers")
	var arr[3] int
	for i:=0; i<3; i++{
		fmt.Scan(&arr[i])
	}
	largest_no:= largest(arr)
	fmt.Println("Largest no:", largest_no)
	var wait string
	fmt.Println("Press Enter to exit..")
	fmt.Scan(&wait)

}