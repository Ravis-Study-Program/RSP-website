package accounts

type AdminProfile struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Slug      string  `json:"slug"`
	AvatarURL *string `json:"avatarUrl"`
	Email     string  `json:"email"`
	DiscordID *string `json:"discordId"`
}
