package kmeans

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"math/rand"
	"os"
	"testing"
	"time"
)

func TestPartitionTwo(t *testing.T) {
	os.Setenv("dev", "true")
	zerolog.TimeFieldFormat = "05.000000000"
	var dd []int32
	for i := 0; i < 200; i++ {
		if i%4 == 0 {
			dd = append(dd, 1000+rand.Int31n(50))
			continue
		}
		dd = append(dd, 10+rand.Int31n(50))
		continue
	}
	for i := 0; i < 1; i++ {
		start := time.Now()
		ret := PartitionTwo(dd)
		for _, r := range ret {
			log.Printf("time:%dus,data:%v\n", time.Since(start).Microseconds(), r)
		}
	}
	var dd2 []int32
	for i := 0; i < 200; i++ {
		dd2 = append(dd2, 1000+rand.Int31n(1000))
		continue
	}
	for i := 0; i < 2; i++ {
		start := time.Now()
		ret := PartitionTwo(dd2)
		for _, r := range ret {
			log.Printf("time:%dus,data:%v\n", time.Since(start).Microseconds(), r)
		}
	}

	var dd3 []int32
	for i := 0; i < 100; i++ {
		dd3 = append(dd3, 200+rand.Int31n(200))
		continue
	}
	for i := 0; i < 100; i++ {
		if i%3 == 0 {
			dd3 = append(dd3, 10000+rand.Int31n(200))
		}
		if i%5 == 0 {
			dd3 = append(dd3, 60000+rand.Int31n(200))
		}
		if i%10 == 0 {
			dd3 = append(dd3, 300000+rand.Int31n(200))
		}
	}
	for i := 0; i < 3; i++ {
		start := time.Now()
		ret := PartitionTwo(dd3)
		for _, r := range ret {
			log.Printf("time:%dus,data:%v\n", time.Since(start).Microseconds(), r)
		}
	}

	for i := 0; i < 3; i++ {
		start := time.Now()
		ret := PartitionV1(dd3)
		for _, r := range ret {
			log.Printf("time:%dus,data:%v\n", time.Since(start).Microseconds(), r)
		}
	}

}
