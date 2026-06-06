package sdk

func Step(label string, fn func()) {
	fn()
}
