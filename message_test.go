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
				Name:    "name1",
				Payload: []byte("value"),
			},
		}, {
			name:    "Simple message with no value",
			args:    append([]byte{0x0, 0x0, 5, 0x0, 0, 0x0, 0}, []byte(`name1`)...),
			wantErr: false,
			want: &Message{
				Name:    "name1",
				Payload: []byte{},
			},
		}, {
			name:    "Simple message with no name and route",
			args:    append([]byte{0x0, 0x0, 0, 0x0, 3, 0x0, 6}, []byte(`RrrValue1`)...),
			wantErr: false,
			want: &Message{
				Name:    "",
				Payload: []byte("Value1"),
				Route:   "Rrr",
			},
		}, {
			name:    "Simple message with some code and route",
			args:    append([]byte{0x1, 0x0, 0, 0x0, 5, 0x0, 6}, []byte(`RouteValue1`)...),
			wantErr: false,
			want: &Message{
				Code:    1,
				Name:    "",
				Payload: []byte("Value1"),
				Route:   "Route",
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
		Name   string
		Value  []byte
		Route  string
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
				Name:  "name1",
				Value: []byte("value"),
			},
		}, {
			name:    "Simple message with no value",
			want:    append([]byte{0x0, 0x0, 5, 0x0, 0, 0x0, 0}, []byte(`name1`)...),
			wantErr: false,
			fields: fields{
				Name:  "name1",
				Value: []byte{},
			},
		}, {
			name:    "Simple message with single char name",
			want:    append([]byte{0x0, 0x0, 1, 0x0, 0, 0x0, 6}, []byte(`nvalue1`)...),
			wantErr: false,
			fields: fields{
				Name:  "n",
				Value: []byte("value1"),
			},
		}, {
			name:    "Simple message with single char name and some action",
			want:    append([]byte{0x1, 0x0, 1, 0x0, 0, 0x0, 6}, []byte(`nvalue1`)...),
			wantErr: false,
			fields: fields{
				Action: 1,
				Name:   "n",
				Value:  []byte("value1"),
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
