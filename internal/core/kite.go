package core

// list of things our core provides any web interface can use
type Kite interface {
	Ask(question string) (string, error)

}
