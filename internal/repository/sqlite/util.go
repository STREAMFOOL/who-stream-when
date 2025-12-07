package sqlite

import "time"

// timeNow is a variable that can be overridden in tests
var timeNow = time.Now
