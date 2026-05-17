package password

import "testing"

func BenchmarkHashDefaultParams(b *testing.B) {
	params := NormalizeParams(0, 0)
	for i := 0; i < b.N; i++ {
		if _, err := Hash("correct horse battery staple", params); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkVerifyDefaultParams(b *testing.B) {
	params := NormalizeParams(0, 0)
	hash, err := Hash("correct horse battery staple", params)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ok, needsRehash := Verify("correct horse battery staple", hash, params)
		if !ok {
			b.Fatal("Verify rejected valid password")
		}
		if needsRehash {
			b.Fatal("Verify unexpectedly requested rehash")
		}
	}
}
