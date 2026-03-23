package repository

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}

	result := "?"
	for i := 1; i < n; i++ {
		result += ", ?"
	}

	return result
}

func int64sToInterfaces(nums []int64) []interface{} {
	interfaces := make([]interface{}, len(nums))
	for i, n := range nums {
		interfaces[i] = n
	}

	return interfaces
}
