package fdstream

import (
	"reflect"
	"testing"
)

func TestUnmarshal(t *testing.T) {

	tests := []struct {
		name    string
		args    []byte
		want    *Message
		wantErr bool
	}{
		{
			name:    "Simple correct message",
			args:    append([]byte{0x0, 0x0, 5, 0x0, 0, 0x0, 5}, []byte(`name1value`)...),
			wantErr: false,
			want: &Message{
				Name:    []byte("name1"),
				Payload: []byte("value"),
				Route:   []byte{},
			},
		}, {
			name:    "Simple message with no value",
			args:    append([]byte{0x0, 0x0, 5, 0x0, 0, 0x0, 0}, []byte(`name1`)...),
			wantErr: false,
			want: &Message{
				Name:    []byte("name1"),
				Payload: []byte{},
				Route:   []byte{},
			},
		}, {
			name:    "Simple message with no name",
			args:    append([]byte{0x0, 0x0, 0, 0x0, 0, 0x0, 6}, []byte(`value1`)...),
			wantErr: false,
			want: &Message{
				Name:    []byte(""),
				Payload: []byte("value1"),
				Route:   []byte{},
			},
		}, {
			name:    "Simple message with some action",
			args:    append([]byte{0x1, 0x0, 0, 0x0, 0, 0x0, 6}, []byte(`value1`)...),
			wantErr: false,
			want: &Message{
				Code:    1,
				Name:    []byte(""),
				Payload: []byte("value1"),
				Route:   []byte{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Unmarshal(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Unmarshal() = %v, want %v", got, tt.want)
			}
		})
	}

}

func TestMessage_Marshal(t *testing.T) {
	type fields struct {
		Action byte
		Name   []byte
		Value  []byte
		Route  []byte
	}
	tests := []struct {
		name    string
		fields  fields
		want    []byte
		wantErr bool
	}{
		{
			name:    "Simple correct message",
			want:    append([]byte{0x0, 0x0, 5, 0x0, 0, 0x0, 5}, []byte(`name1value`)...),
			wantErr: false,
			fields: fields{
				Name:  []byte("name1"),
				Value: []byte("value"),
				Route: []byte{},
			},
		}, {
			name:    "Simple message with no value",
			want:    append([]byte{0x0, 0x0, 5, 0x0, 0, 0x0, 0}, []byte(`name1`)...),
			wantErr: false,
			fields: fields{
				Name:  []byte("name1"),
				Value: []byte{},
				Route: []byte{},
			},
		}, {
			name:    "Simple message with single char name",
			want:    append([]byte{0x0, 0x0, 1, 0x0, 0, 0x0, 6}, []byte(`nvalue1`)...),
			wantErr: false,
			fields: fields{
				Name:  []byte("n"),
				Value: []byte("value1"),
				Route: []byte{},
			},
		}, {
			name:    "Simple message with single char name and some action",
			want:    append([]byte{0x1, 0x0, 1, 0x0, 0, 0x0, 6}, []byte(`nvalue1`)...),
			wantErr: false,
			fields: fields{
				Action: 1,
				Name:   []byte("n"),
				Value:  []byte("value1"),
				Route:  []byte{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Message{
				Code:    tt.fields.Action,
				Name:    tt.fields.Name,
				Payload: tt.fields.Value,
				Route:   tt.fields.Route,
			}
			got, err := m.Marshal()
			if (err != nil) != tt.wantErr {
				t.Errorf("Message.Marshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Message.Marshal() = %v, want %v", got, tt.want)
			}
		})
	}
}

//TODO bench
