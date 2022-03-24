package moon

type processor interface {
	process(request ProposeInfoRequest)
}
