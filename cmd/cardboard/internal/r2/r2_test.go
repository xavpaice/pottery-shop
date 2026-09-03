package r2

import (
	"testing"
)

func TestDeriveTrackInfo(t *testing.T) {
	cases := []struct {
		name           string
		key            string
		meta           map[string]string
		wantArtist     string
		wantTitle      string
		wantAlbum      string
	}{
		{
			name:       "filename artist-title",
			key:        "Artist - Title.mp3",
			meta:       map[string]string{},
			wantArtist: "Artist",
			wantTitle:  "Title",
			wantAlbum:  "",
		},
		{
			name:       "album directory artist-title",
			key:        "Album/Artist - Title.mp3",
			meta:       map[string]string{},
			wantArtist: "Artist",
			wantTitle:  "Title",
			wantAlbum:  "Album",
		},
		{
			name:       "title only",
			key:        "Title.mp3",
			meta:       map[string]string{},
			wantArtist: "",
			wantTitle:  "Title",
			wantAlbum:  "",
		},
		{
			name:       "metadata overrides filename",
			key:        "Artist - Title.mp3",
			meta:       map[string]string{"Artist": "MetaArtist", "Title": "MetaTitle", "Album": "MetaAlbum"},
			wantArtist: "MetaArtist",
			wantTitle:  "MetaTitle",
			wantAlbum:  "MetaAlbum",
		},
		{
			name:       "metadata case insensitive",
			key:        "file.mp3",
			meta:       map[string]string{"ARTIST": "Upper", "Title": "Mixed", "ALBUM": "ALBUM"},
			wantArtist: "Upper",
			wantTitle:  "Mixed",
			wantAlbum:  "ALBUM",
		},
		{
			name:       "partial metadata uses filename for rest",
			key:        "FileArtist - FileTitle.mp3",
			meta:       map[string]string{"title": "MetaTitle"},
			wantArtist: "FileArtist",
			wantTitle:  "MetaTitle",
			wantAlbum:  "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			artist, title, album := deriveTrackInfo(tc.key, normalizeMetadata(tc.meta))
			if artist != tc.wantArtist {
				t.Errorf("artist = %q, want %q", artist, tc.wantArtist)
			}
			if title != tc.wantTitle {
				t.Errorf("title = %q, want %q", title, tc.wantTitle)
			}
			if album != tc.wantAlbum {
				t.Errorf("album = %q, want %q", album, tc.wantAlbum)
			}
		})
	}
}

func TestBuildPublicURL(t *testing.T) {
	cases := []struct {
		prefix string
		key    string
		want   string
	}{
		{"https://example.com", "song.mp3", "https://example.com/song.mp3"},
		{"https://example.com/", "song.mp3", "https://example.com/song.mp3"},
		{"https://example.com", "album/song.mp3", "https://example.com/album/song.mp3"},
		{"https://example.com/", "/album/song.mp3", "https://example.com/album/song.mp3"},
	}

	for _, tc := range cases {
		got := buildPublicURL(tc.prefix, tc.key)
		if got != tc.want {
			t.Errorf("buildPublicURL(%q, %q) = %q, want %q", tc.prefix, tc.key, got, tc.want)
		}
	}
}

func TestNormalizeMetadata(t *testing.T) {
	in := map[string]string{"TITLE": "Song", "Artist": "Band", "ALBUM": "LP"}
	out := normalizeMetadata(in)
	if out["title"] != "Song" || out["artist"] != "Band" || out["album"] != "LP" {
		t.Errorf("normalizeMetadata did not lowercase keys correctly: %v", out)
	}
}
