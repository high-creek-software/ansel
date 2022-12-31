package ansel

type unit[I comparable] struct {
	id     I
	source string
	target ResourceSetter
}
