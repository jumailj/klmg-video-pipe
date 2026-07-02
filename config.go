package main

// TURNConfig contains TURN server configuration for WebRTC connectivity
type TURNConfig struct {
	Servers []map[string]interface{} `json:"servers"`
}

// GetTURNConfig returns public TURN servers (Google and Twilio)
// These work across networks without needing a private TURN server
func GetTURNConfig() TURNConfig {
	return TURNConfig{
		Servers: []map[string]interface{}{
			// Google STUN (free, no auth)
			{
				"urls": []string{
					"stun:stun.l.google.com:19302",
					"stun:stun1.l.google.com:19302",
					"stun:stun2.l.google.com:19302",
					"stun:stun3.l.google.com:19302",
					"stun:stun4.l.google.com:19302",
				},
			},
			// Bistri TURN
			{
				"urls": []string{
					"turn:turn.bistri.com:80",
					"turn:turn.bistri.com:443?transport=tcp",
				},
				"username":   "webrtc",
				"credential": "webrtc",
			},
			// Twilio TURN (reliable)
			{
				"urls": []string{
					"turn:turnserver.twilio.com:443?transport=tcp",
					"turn:turnserver.twilio.com:443?transport=udp",
				},
				"username":   "webrtc",
				"credential": "webrtc",
			},
			// Open Relay Project (fallback)
			{
				"urls": []string{
					"turn:openrelay.metered.ca:80",
					"turn:openrelay.metered.ca:443?transport=tcp",
				},
				"username":   "openrelayproject",
				"credential": "openrelayproject",
			},
		},
	}
}
