package kmeans

import (
	m2 "github.com/AliyunContainerService/scaler/go/pkg/model"
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
	for i := 0; i < 1; i++ {
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
	for i := 0; i < 1; i++ {
		start := time.Now()
		ret := PartitionTwo(dd3)
		for _, r := range ret {
			log.Printf("time:%dus,data:%v\n", time.Since(start).Microseconds(), r)
		}
	}
	//for i := 0; i < 100; i++ {
	//	if i%3 == 0 {
	//		dd3 = append(dd3, 10000+rand.Int31n(200))
	//	}
	//	if i%5 == 0 {
	//		dd3 = append(dd3, 60000+rand.Int31n(200))
	//	}
	//	if i%10 == 0 {
	//		dd3 = append(dd3, 300000+rand.Int31n(200))
	//	}
	//}
	//for i := 0; i < 1; i++ {
	//	start := time.Now()
	//	ret := PartitionTwo(dd3)
	//	for _, r := range ret {
	//		log.Printf("time:%dus,data:%v\n", time.Since(start).Microseconds(), r)
	//	}
	//}
	//
	//for i := 0; i < 1; i++ {
	//	start := time.Now()
	//	ret := PartitionV1(dd3)
	//	for _, r := range ret {
	//		log.Printf("time:%dus,data:%v\n", time.Since(start).Microseconds(), r)
	//	}
	//}
}

func TestTime(t *testing.T) {
	zerolog.TimeFieldFormat = "05.000000000"
	it := new([200]int32)
	reqCnt := 100
	for i := 0; i < reqCnt; i++ {
		if i%5 == 0 {
			it[i%200] = 5000 + rand.Int31n(200)
		} else {
			it[i%200] = rand.Int31n(200)
		}
	}
	part(it, reqCnt)
}

func part(ITData *[200]int32, ReqCnt int) {
	rawData := ITData[:]
	maxIt := int32(0)
	sumIt := int32(0)
	for _, n := range rawData {
		if n > maxIt {
			maxIt = n
		}
		sumIt += n
	}
	var data []int32
	// 取最大周期五倍的数据
	dur := 5 * maxIt
	if dur < sumIt && sumIt > 70000 {
		if dur < 70000 {
			dur = 70000
		}
		req := ReqCnt - 1
		start := req - 200
		if start < 0 {
			start = -1
		}
		for dur > 0 && req > start {
			data = append(data, rawData[req%len(rawData)])
			dur -= rawData[req%len(rawData)]
			req -= 1
		}
	} else {
		if ReqCnt < 200 {
			rawData = rawData[:ReqCnt-1]
		}
		data = rawData
	}
	cs := PartitionV1(data)
	m2.Printf("partResult:%s,%v,%v,%v", "sss", cs[0], cs[1], data)
}

func Test2(t *testing.T) {
	zerolog.TimeFieldFormat = "05.000000000"
	var dd []int32
	dd = append(dd, 10, 10)
	for i := 0; i < 1; i++ {
		start := time.Now()
		ret := PartitionTwo(dd)
		for _, r := range ret {
			log.Printf("time:%dus,data:%v\n", time.Since(start).Microseconds(), r)
		}
	}
}
