package main

import (
	"encoding/json"
	"math"
	"time"

	"github.com/Zamiell/isaac-tournament-bot/src/models"
	"github.com/bwmarrin/discordgo"
)

func commandStartRound(m *discordgo.MessageCreate, args []string) {
	if !isAdmin(m) {
		return
	}

	// Go through all of the tournaments
	for _, tournament := range tournaments {
		startRound(m, tournament, false)
	}
}

func startRound(m *discordgo.MessageCreate, tournament Tournament, dryRun bool) {
	// Get the tournament from Challonge
	apiURL := "https://api.challonge.com/v1/tournaments/" + floatToString(tournament.ChallongeID) + ".json?"
	apiURL += "api_key=" + challongeAPIKey + "&include_participants=1&include_matches=1"
	var raw []byte
	if v, err := challongeGetJSON("GET", apiURL, nil); err != nil {
		msg := "Failed to get the tournament from Challonge: " + err.Error()
		log.Error(msg)
		discordSend(m.ChannelID, msg)
		return
	} else {
		raw = v
	}

	vMap := make(map[string]interface{})
	if err := json.Unmarshal(raw, &vMap); err != nil {
		msg := "Failed to unmarshal the Challonge JSON: " + err.Error()
		log.Error(msg)
		discordSend(m.ChannelID, msg)
		return
	}
	jsonTournament := vMap["tournament"].(map[string]interface{})
	var roles []*discordgo.Role
	if v, err := discord.GuildRoles(discordGuildID); err != nil {
		log.Fatal("Failed to get the roles for the guild: " + err.Error())
		return
	} else {
		roles = v
	}

	// Get all of the open matches
	foundMatches := false
	var round string
	for _, v := range jsonTournament["matches"].([]interface{}) {
		vMap := v.(map[string]interface{})
		match := vMap["match"].(map[string]interface{})
		if match["state"] != "open" {
			continue
		}

		// Local variables
		foundMatches = true
		player1ID := match["player1_id"].(float64)
		player2ID := match["player2_id"].(float64)
		player1Name := challongeGetParticipantName(jsonTournament, player1ID)
		player2Name := challongeGetParticipantName(jsonTournament, player2ID)
		round = floatToString(match["round"].(float64))
		challongeMatchID := floatToString(match["id"].(float64))
		channelName := player1Name + "-vs-" + player2Name

		// Get the Discord guild object
		var guild *discordgo.Guild
		if v, err := discord.Guild(discordGuildID); err != nil {
			msg := "Failed to get the Discord guild: " + err.Error()
			log.Error(msg)
			discordSend(m.ChannelID, msg)
			return
		} else {
			guild = v
		}

		// Find the discord ID of the two players and add them to the database if they are not already
		var racer1 models.Racer
		var racer2 models.Racer
		var team1DiscordID string
		var team2DiscordID string
		var discord1 *discordgo.User
		var discord2 *discordgo.User
		if tournament.Ruleset == "team" {
			for _, role := range roles {
				if role.Name == player1Name {
					team1DiscordID = role.ID
				}
			}
			for _, role := range roles {
				if role.Name == player2Name {
					team2DiscordID = role.ID
				}
			}
			for _, member := range guild.Members {
				if stringInSlice(discordTeamCaptainRoleID, member.Roles) && stringInSlice(team1DiscordID, member.Roles) {
					discord1 = member.User
					break
				}
			}

			if discord1 == nil {
				msg := "Failed to find \"" + player1Name + "\" team captain in the Discord server."
				log.Error(msg)
				discordSend(m.ChannelID, msg)
				return
			}
			for _, member := range guild.Members {
				if stringInSlice(discordTeamCaptainRoleID, member.Roles) && stringInSlice(team2DiscordID, member.Roles) {
					discord2 = member.User
					break
				}
			}
			if discord2 == nil {
				msg := "Failed to find \"" + player2Name + "\" team captain in the Discord server."
				log.Error(msg)
				discordSend(m.ChannelID, msg)
				return
			}
			if v, err := racerGet(discord1); err != nil {
				msg := "Failed to get the racer from the database: " + err.Error()
				log.Error(msg)
				discordSend(m.ChannelID, msg)
				return
			} else {
				racer1 = v
			}
			if v, err := racerGet(discord2); err != nil {
				msg := "Failed to get the racer from the database: " + err.Error()
				log.Error(msg)
				discordSend(m.ChannelID, msg)
				return
			} else {
				racer2 = v
			}
		} else {
			for _, member := range guild.Members {
				username := member.Nick
				if username == "" {
					username = member.User.Username
				}
				if username == player1Name {
					discord1 = member.User
					break
				}
			}
			if discord1 == nil {
				msg := "Failed to find \"" + player1Name + "\" in the Discord server."
				log.Error(msg)
				discordSend(m.ChannelID, msg)
				return
			}
			for _, member := range guild.Members {
				username := member.Nick
				if username == "" {
					username = member.User.Username
				}
				if username == player2Name {
					discord2 = member.User
					break
				}
			}
			if discord2 == nil {
				msg := "Failed to find \"" + player2Name + "\" in this Discord server."
				log.Error(msg)
				discordSend(m.ChannelID, msg)
				return
			}
			if v, err := racerGet(discord1); err != nil {
				msg := "Failed to get the racer from the database: " + err.Error()
				log.Error(msg)
				discordSend(m.ChannelID, msg)
				return
			} else {
				racer1 = v
			}
			if v, err := racerGet(discord2); err != nil {
				msg := "Failed to get the racer from the database: " + err.Error()
				log.Error(msg)
				discordSend(m.ChannelID, msg)
				return
			} else {
				racer2 = v
			}
		}

		if dryRun {
			continue
		}

		// Create a channel for this match
		var channelID string
		if v, err := discord.GuildChannelCreate(discordGuildID, channelName, "text"); err != nil {
			msg := "Failed to create the Discord channel of \"" + channelName + "\": " + err.Error()
			log.Error(msg)
			discordSend(m.ChannelID, msg)
			return
		} else {
			channelID = v.ID
		}

		// Create the race in the database
		race := models.Race{
			TournamentName:      tournament.Name,
			Racer1ChallongeID:   player1ID,
			Racer2ChallongeID:   player2ID,
			ChannelID:           channelID,
			ChallongeURL:        tournament.ChallongeURL,
			ChallongeMatchID:    challongeMatchID,
			BracketRound:        round,
			State:               "initial",
			CharactersRemaining: characters,
			BuildsRemaining:     builds,
			Racer1Bans:          numBans,
			Racer2Bans:          numBans,
			Racer1Vetos:         numVetos,
			Racer2Vetos:         numVetos,
		}
		if err := db.Races.Insert(racer1.DiscordID, racer2.DiscordID, race); err != nil {
			msg := "Failed to create the race in the database: " + err.Error()
			log.Error(msg)
			discordSend(m.ChannelID, msg)
			return
		}

		// Put the channel in the correct category and give access to the two racers
		// (channels in this category have "Read Text Channels & See Voice Channels" disabled for everyone except for admins/casters/bots)
		permissionsReadWrite := discordgo.PermissionReadMessages |
			discordgo.PermissionSendMessages |
			discordgo.PermissionEmbedLinks |
			discordgo.PermissionAttachFiles |
			discordgo.PermissionReadMessageHistory
		var permissions = make([]*discordgo.PermissionOverwrite, 0)
		permissions = append(permissions,
			&discordgo.PermissionOverwrite{
				ID:   discordEveryoneRoleID,
				Type: "role",
				Deny: permissionsReadWrite,
			},

			// Allow bots to see + talk in this channel
			&discordgo.PermissionOverwrite{
				ID:    discordBotRoleID,
				Type:  "role",
				Allow: permissionsReadWrite,
			},

			// Allow all casters to see + talk in this channel
			&discordgo.PermissionOverwrite{
				ID:    discordCasterRoleID,
				Type:  "role",
				Allow: permissionsReadWrite,
			})
		if tournament.Ruleset == "team" {
			permissions = append(permissions,
				&discordgo.PermissionOverwrite{
					ID:    team1DiscordID,
					Type:  "role",
					Allow: permissionsReadWrite,
				},
				&discordgo.PermissionOverwrite{
					ID:    team2DiscordID,
					Type:  "role",
					Allow: permissionsReadWrite,
				})
		} else {
			permissions = append(permissions,
				&discordgo.PermissionOverwrite{
					ID:    racer1.DiscordID,
					Type:  "member",
					Allow: permissionsReadWrite,
				},
				&discordgo.PermissionOverwrite{
					ID:    racer2.DiscordID,
					Type:  "member",
					Allow: permissionsReadWrite,
				})
		}
		discord.ChannelEditComplex(channelID, &discordgo.ChannelEdit{
			PermissionOverwrites: permissions,
			ParentID:             tournament.DiscordCategoryID,
		})
		// Find out if the players have set their timezone
		msg := ""
		if racer1.Timezone.Valid {
			msg += discord1.Mention() + " has a timezone of: " + getTimezone(racer1.Timezone.String) + "\n"
		} else {
			msg += discord1.Mention() + ", your timezone is **not currently set**. Please set one with: `!timezone [timezone]`\n"
		}
		if racer2.Timezone.Valid {
			msg += discord2.Mention() + " has a timezone of: " + getTimezone(racer2.Timezone.String) + "\n"
		} else {
			msg += discord2.Mention() + ", your timezone is **not currently set**. Please set one with: `!timezone [timezone]`\n"
		}

		// Calculate the difference between the two timezones
		if racer1.Timezone.Valid && racer2.Timezone.Valid {
			loc1, _ := time.LoadLocation(racer1.Timezone.String)
			loc2, _ := time.LoadLocation(racer2.Timezone.String)
			_, offset1 := time.Now().In(loc1).Zone()
			_, offset2 := time.Now().In(loc2).Zone()
			if offset1 == offset2 {
				msg += "You both are in **the same timezone**. Great!\n"
			} else {
				difference := math.Abs(float64(offset1 - offset2))
				hours := difference / 3600
				msg += "You are **" + floatToString(hours) + " hours** away from each other.\n"
			}
		}
		msg += "\n"

		// Find out if the players have set their stream URL
		if racer1.StreamURL.Valid {
			msg += discord1.Mention() + " has a stream of: <" + racer1.StreamURL.String + ">\n"
		} else {
			msg += discord1.Mention() + ", your stream is **not currently set**. Please set one with: `!stream [url]`\n"
		}
		if racer2.StreamURL.Valid {
			msg += discord2.Mention() + " has a stream of: <" + racer2.StreamURL.String + ">\n"
		} else {
			msg += discord2.Mention() + ", your stream is **not currently set**. Please set one with: `!stream [url]`\n"
		}
		msg += "\n"

		// Give the welcome message
		msg += "Please discuss the times that each of you are available to play this week.\n"
		if tournament.Ruleset == "team" {
			msg += discord1.Mention() + " and " + discord2.Mention() + " are the team captains, only them can operate the bot and submit times agreed upon.\n"
		}
		msg += "You can use suggest a time to your opponent with something like: `!time 6pm sat`\n"
		msg += "If they accept with `!timeok`, then the match will be officially scheduled."
		discordSend(channelID, msg)

		log.Info("Started race: " + channelName)
	}

	if dryRun {
		msg := "Tournament \"" + tournament.Name + "\" looks good."
		discordSend(m.ChannelID, msg)
		log.Info(msg)
		return
	}

	// Rename the channel category
	categoryName := "Round " + round + " - " + tournament.Ruleset
	discord.ChannelEdit(tournament.DiscordCategoryID, categoryName)

	if foundMatches {
		msg := "Round " + round + " channels created for tournament \"" + tournament.Name + "\"."
		discordSend(m.ChannelID, msg)
		log.Info(msg)
	} else {
		msg := "There are no open matches on the Challonge bracket for tournament \"" + tournament.Name + "\"."
		discordSend(m.ChannelID, msg)
		log.Info(msg)
	}
}
