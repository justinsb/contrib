package fi

/* SystemUnits are not templated*/
type SystemUnit struct {
}

type SystemUnitInterface interface {
	IsSystemUnit() bool
}

func (u *SystemUnit) IsSystemUnit() bool {
	return true
}
