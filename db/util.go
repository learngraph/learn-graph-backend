package db

// All returns true if all entries of `ar` return true from the predicate `pred`
func All[T any, A ~[]T](ar A, pred func(T) bool) bool {
	for _, a := range ar {
		if !pred(a) {
			return false
		}
	}
	return true
}

// FindFirst finds the first T that satisfies predicate `pred` in `ar`, returns
// nil if none is found
func FindFirst[T any, A ~[]T](ar A, pred func(t T) bool) *T {
	for _, t := range ar {
		if pred(t) {
			return &t
		}
	}
	return nil
}
