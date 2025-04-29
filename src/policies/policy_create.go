package policies

func CreateCommand(name string) error {
	_, err := Create(name)
	return err
}

func Create(name string) (*Policy, error) {
	dir, err := PolicyHomeDir(name)
	if err != nil {
		return nil, err
	}

	p := &Policy{}
	return p, p.WriteToDir(dir)
}
