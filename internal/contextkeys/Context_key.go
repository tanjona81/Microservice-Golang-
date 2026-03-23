package contextkeys

// this custom type is used to avoid naming collisions
// contextKey is private so no other package can accidentally overwrite the data
type contextKey string

const UserIDKey contextKey = "userId"
const RequestIDKey contextKey = "RequestId"
const ClaimedIDKey contextKey = "ClaimedId"
