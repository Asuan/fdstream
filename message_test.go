package fdstream

import (
	"io/ioutil"
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
			args:    append([]byte{0x0, 0, 0, 0, 0, 0x0, 5, 0x0, 5}, []byte(`name1value`)...),
			wantErr: false,
			want: &Message{
				Name:    "name1",
				Payload: []byte("value"),
			},
		}, {
			name:    "Simple message with no value",
			args:    append([]byte{0x0, 0, 0, 0, 0, 0x0, 5, 0x0, 0}, []byte(`name1`)...),
			wantErr: false,
			want: &Message{
				Name:    "name1",
				Payload: []byte{},
			},
		}, {
			name:    "Simple message with no name",
			args:    append([]byte{0x0, 0, 0, 0, 0, 0x0, 0, 0x0, 6}, []byte(`Value1`)...),
			wantErr: false,
			want: &Message{
				Name:    "",
				Payload: []byte("Value1"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := unmarshal(tt.args)
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
	}
	tests := []struct {
		name    string
		fields  fields
		want    []byte
		wantErr bool
	}{
		{
			name:    "Simple correct message",
			want:    append([]byte{0x0, 0, 0, 0, 0, 0x0, 5, 0x0, 5}, []byte(`name1value`)...),
			wantErr: false,
			fields: fields{
				Name:  "name1",
				Value: []byte("value"),
			},
		}, {
			name:    "Simple message with no value",
			want:    append([]byte{0x0, 0, 0, 0, 0, 0x0, 5, 0x0, 0}, []byte(`name1`)...),
			wantErr: false,
			fields: fields{
				Name:  "name1",
				Value: []byte{},
			},
		}, {
			name:    "Simple message with single char name",
			want:    append([]byte{0x0, 0, 0, 0, 0, 0x0, 1, 0x0, 6}, []byte(`nvalue1`)...),
			wantErr: false,
			fields: fields{
				Name:  "n",
				Value: []byte("value1"),
			},
		}, {
			name:    "Simple message with single char name and some action",
			want:    append([]byte{0x1, 0, 0, 0, 0, 0x0, 1, 0x0, 6}, []byte(`nvalue1`)...),
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
				Id:      0,
				Name:    tt.fields.Name,
				Payload: tt.fields.Value,
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
		{"name-5", &Message{string(make([]byte, 5, 5)), 0, 0, payload}},
		{"name-10", &Message{string(make([]byte, 10, 10)), 0, 0, payload}},
		{"name-20", &Message{string(make([]byte, 20, 20)), 0, 0, payload}},
		{"name-50", &Message{string(make([]byte, 50, 50)), 0, 0, payload}},
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
		z, _ := (&Message{string(make([]byte, l, l)), 0, 0, payload}).Marshal()
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
				unmarshal(bm.message)
			}
		})
	}
}

func testBenchWriteTo(length int, b *testing.B) {
	b.StopTimer()
	payload := make([]byte, length, length)

	benchmarks := []struct {
		name string

		message *Message
	}{
		{"name-10", &Message{string(make([]byte, 10, 10)), 0, 0, payload}},
		{"name-20", &Message{string(make([]byte, 20, 20)), 0, 0, payload}},
		{"name-50", &Message{string(make([]byte, 50, 50)), 0, 0, payload}},
		{"name-5", &Message{string(make([]byte, 5, 5)), 0, 0, payload}},
	}
	b.StartTimer()
	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				bm.message.WriteTo(ioutil.Discard)
			}
		})
	}
}

func Benchmark_Marshal2(b *testing.B)    { testBenchMarshal(2, b) }
func Benchmark_Marshal50(b *testing.B)   { testBenchMarshal(50, b) }
func Benchmark_Marshal500(b *testing.B)  { testBenchMarshal(500, b) }
func Benchmark_Marshal5000(b *testing.B) { testBenchMarshal(5000, b) }

func Benchmark_Unmarshal2(b *testing.B)    { testBenchUnmarshal(2, b) }
func Benchmark_Unmarshal50(b *testing.B)   { testBenchUnmarshal(50, b) }
func Benchmark_Unmarshal500(b *testing.B)  { testBenchUnmarshal(500, b) }
func Benchmark_Unmarshal5000(b *testing.B) { testBenchUnmarshal(5000, b) }

func Benchmark_WriteTo2(b *testing.B)    { testBenchWriteTo(2, b) }
func Benchmark_WriteTo50(b *testing.B)   { testBenchWriteTo(50, b) }
func Benchmark_WriteTo500(b *testing.B)  { testBenchWriteTo(500, b) }
func Benchmark_WriteTo5000(b *testing.B) { testBenchWriteTo(5000, b) }
