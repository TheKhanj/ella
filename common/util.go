package common

func WaitFor[T comparable](ch <-chan T, val T) bool {
	for curr := range ch {
		if curr == val {
			return true
		}
	}

	return false
}
