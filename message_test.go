package fdstream

import (
	"reflect"
	"testing"
)

func Test_unmarshal(t *testing.T) {

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
func Test_marshal(t *testing.T) {
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

func testBenchMarshal(length int, b *testing.B) {
	b.StopTimer()
	payload := make([]byte, length, length)

	benchmarks := []struct {
		name string

		message *Message
	}{
		{"name-10", &Message{string(make([]byte, 10, 10)), "", 0, payload}},
		{"name-20", &Message{string(make([]byte, 20, 20)), "", 0, payload}},
		{"name-50", &Message{string(make([]byte, 50, 50)), "", 0, payload}},
		{"name-5", &Message{string(make([]byte, 5, 5)), "", 0, payload}},
	}
	b.StartTimer()
	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				bm.message.Marshal()
			}
		})
	}
}

func testBenchUnmarshal(length int, b *testing.B) {
	b.StopTimer()
	payload := make([]byte, length, length)

	stubMessage := func(l int) []byte {
		z, _ := (&Message{string(make([]byte, l, l)), "", 0, payload}).Marshal()
		return z
	}
	benchmarks := []struct {
		name string

		message []byte
	}{
		{"name-10", stubMessage(10)},
		{"name-20", stubMessage(20)},
		{"name-50", stubMessage(50)},
		{"name-5", stubMessage(5)},
	}
	b.StartTimer()
	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				Unmarshal(bm.message)
			}
		})
	}
}

func Benchmark_Marshal2(b *testing.B)    { testBenchMarshal(2, b) }
func Benchmark_Marshal10(b *testing.B)   { testBenchMarshal(10, b) }
func Benchmark_Marshal100(b *testing.B)  { testBenchMarshal(100, b) }
func Benchmark_Marshal1000(b *testing.B) { testBenchMarshal(1000, b) }

func Benchmark_Unmarshal2(b *testing.B)    { testBenchUnmarshal(2, b) }
func Benchmark_Unmarshal10(b *testing.B)   { testBenchUnmarshal(10, b) }
func Benchmark_Unmarshal100(b *testing.B)  { testBenchUnmarshal(100, b) }
func Benchmark_Unmarshal1000(b *testing.B) { testBenchUnmarshal(1000, b) }
