package strings

import "strconv"

const DC_TIME_FORMAT = "15:04"

const DC_LONG_TIME_FORMAT = "2006-01-02 15:04"

func StrToInt64(i string) (int64, error) {
	id, err := strconv.ParseInt(i, 10, 0)
	if err != nil {
		return 0, err
	}

	return id, nil
}
