package main

import (
	"github.com/bwmarrin/discordgo"
)

func commandSetCasterNotOk(m *discordgo.MessageCreate, args []string) {
	if !isAdmin(m) {
		return
	}

	if len(args) == 0 {
		commandSetCasterNotOkPrint(m)
		return
	}
	username := args[0]

	// Get the Discord guild members
	var members []*discordgo.Member
	if v, err := discordSession.GuildMembers(discordGuildID, "0", 1000); err != nil {
		msg := "Failed to get the Discord guild members: " + err.Error()
		log.Error(msg)
		discordSend(m.ChannelID, msg)
		return
	} else {
		members = v
	}

	discordUser := getDiscordUserByName(members, username)
	if discordUser == nil {
		msg := "Failed to find \"" + username + "\" in the Discord server."
		log.Error(msg)
		discordSend(m.ChannelID, msg)
		return
	}

	m.Author = discordUser
	args = args[1:] // This will be an empty slice if there is nothing after the command
	commandCasterNotOk(m, args)
}

func commandSetCasterNotOkPrint(m *discordgo.MessageCreate) {
	msg := "Deny permission on behalf of a racer with: `!setcasternotok [username]`\n"
	msg += "e.g. `!setcasternotok Willy`"
	discordSend(m.ChannelID, msg)
}
