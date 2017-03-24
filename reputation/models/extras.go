package models

func (conf *ReputationConfig) GetPointsName() string {
	if conf.PointsName != "" {
		return conf.PointsName
	}

	return "Rep"
}
