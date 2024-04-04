package strings

import "strconv"

const DcTimeFormat = "15:04"

const DcLongTimeFormat = "2006-01-02 15:04"

// Periodic message to be sent in the command channel.
const PeriodicMessageContent = "If you want to support development, consider buying me a coffee: <https://www.buymeacoffee.com/marahin> or contributing to the open source code: <https://github.com/marahin/letter-bot>"

func StrToInt64(i string) (int64, error) {
	id, err := strconv.ParseInt(i, 10, 0)
	if err != nil {
		return 0, err
	}

	return id, nil
}
