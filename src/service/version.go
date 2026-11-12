package service

import (
	"encoding/json"
	"errors"
	"time"
)

type ServiceVersion struct {
	time.Time
}

const ServiceVersionFormat = "2006-01-02 15"
const ServiceVersionSFormat = "2006-01-02"
const ServiceVersionXSFormat = "2006-01"
const ServiceVersionXXSFormat = "2006"

var DefaultVersion time.Time = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func (t ServiceVersion) MarshalJSON() ([]byte, error) {
	b := make([]byte, 0, len(ServiceVersionFormat)+len(`""`))
	b = append(b, '"')
	if t.Hour() == 0 {
		b = t.AppendFormat(b, ServiceVersionSFormat)
	} else {
		b = t.AppendFormat(b, ServiceVersionFormat)
	}
	b = append(b, '"')
	return b, nil
}

func (t *ServiceVersion) UnmarshalJSON(data []byte) error {
	*t = ServiceVersion{DefaultVersion}

	if string(data) == "null" {
		return nil
	}
	var date string
	err := json.Unmarshal(data, &date)
	if err != nil {
		return errors.New("ServiceVersion.UnmarshalJSON: " + err.Error())
	}

	var parsed time.Time = DefaultVersion
	if len(date) == 4 {
		parsed, err = time.Parse(ServiceVersionXXSFormat, date)
	} else if len(date) == 7 {
		parsed, err = time.Parse(ServiceVersionXSFormat, date)
	} else if len(date) == 10 {
		parsed, err = time.Parse(ServiceVersionSFormat, date)
	} else {
		parsed, err = time.Parse(ServiceVersionFormat, date)
	}
	if err != nil {
		return err
	}

	*t = ServiceVersion{parsed}

	return nil
}
