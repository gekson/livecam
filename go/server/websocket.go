package server

import (
	"github.com/pion/webrtc/v3"
)

type Stream struct {
    PeerConnection *webrtc.PeerConnection
    VideoTrack     *webrtc.TrackLocalStaticRTP
}

func NewStream() (*Stream, error) {
    peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{})
    if err != nil {
        return nil, err
    }

    videoTrack, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: "video/vp8"}, "video", "stream")
    if err != nil {
        return nil, err
    }

    if _, err = peerConnection.AddTrack(videoTrack); err != nil {
        return nil, err
    }

    return &Stream{
        PeerConnection: peerConnection,
        VideoTrack:     videoTrack,
    }, nil
}
