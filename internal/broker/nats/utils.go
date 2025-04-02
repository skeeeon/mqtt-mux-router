package nats

import (
	"strings"
)

// ToNATSSubject converts an MQTT topic format to NATS subject format
// MQTT uses / as separators and +/# as wildcards
// NATS uses . as separators and */> as wildcards
func ToNATSSubject(mqttTopic string) string {
	// First handle wildcards
	subject := strings.ReplaceAll(mqttTopic, "+", "*")
	subject = strings.ReplaceAll(subject, "#", ">")
	
	// Then handle separators
	subject = strings.ReplaceAll(subject, "/", ".")
	
	return subject
}

// ToMQTTTopic converts a NATS subject format to MQTT topic format
// This is the reverse of ToNATSSubject
func ToMQTTTopic(natsSubject string) string {
	// First handle wildcards
	topic := strings.ReplaceAll(natsSubject, "*", "+")
	topic = strings.ReplaceAll(topic, ">", "#")
	
	// Then handle separators
	topic = strings.ReplaceAll(topic, ".", "/")
	
	return topic
}

// NormalizeSubject ensures a NATS subject doesn't have invalid characters
func NormalizeSubject(subject string) string {
	// NATS subjects can't have spaces or certain special characters
	// For now, just do a simple replace of common problematic characters
	replacer := strings.NewReplacer(
		" ", "_",
		",", "_",
		":", "_",
		"?", "_",
		"[", "_",
		"]", "_",
	)
	return replacer.Replace(subject)
}
