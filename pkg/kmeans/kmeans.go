package kmeans

import (
	"math/rand"
	"time"
)

var deltaThreshold = 0.01
var IterationThreshold = 96

type Cluster struct {
	Center int32
	Max    int32
	Min    int32
	Diff   int32
	Points []int32
}

func NewCluster(dataLen int) *Cluster {
	rand.Seed(time.Now().UnixNano())
	return &Cluster{
		Center: rand.Int31n(60000),
		Points: make([]int32, 0),
	}
}

func Nearest(cls *[2]*Cluster, point int32) int {
	curDis := cls[0].Center - point
	if curDis < 0 {
		curDis = -curDis
	}
	nextDis := cls[1].Center - point
	if nextDis < 0 {
		nextDis = -nextDis
	}
	if nextDis > curDis {
		return 0
	}
	return 1
}

func PartitionV1(data []int32) *[2]*Cluster {
	cc := PartitionTwo(data)
	for cc[0].Max > cc[0].Min*20 && cc[0].Max > 20000 {
		cc2 := PartitionTwo(cc[0].Points)
		cc[0] = cc2[0]
		cc[1] = Merge(cc2[1], cc[1])
	}
	return cc
}

func Merge(c1 *Cluster, c2 *Cluster) *Cluster {
	for _, c := range c1.Points {
		c2.Points = append(c2.Points, c)
	}
	Recenter(c2)
	return c2
}

func PartitionTwo(dataset []int32) *[2]*Cluster {
	cc := new([2]*Cluster)
	cc[0] = NewCluster(len(dataset))
	cc[1] = NewCluster(len(dataset))
	points := make([]int, len(dataset))
	changes := 1
	for i := 0; changes > 0; i++ {
		changes = 0
		for _, c := range cc {
			c.Points = make([]int32, 0)
		}
		for p, point := range dataset {
			ci := Nearest(cc, point)
			cc[ci].Points = append(cc[ci].Points, point)
			if points[p] != ci {
				points[p] = ci
				changes++
			}
		}

		for ci := 0; ci < len(cc); ci++ {
			if len(cc[ci].Points) == 0 {
				var ri int
				for {
					// find a cluster with at least two data points, otherwise
					// we're just emptying one cluster to fill another
					ri = rand.Intn(len(dataset)) //nolint:gosec // rand.Intn is good enough for this
					if len(cc[points[ri]].Points) > 1 {
						break
					}
				}
				cc[ci].Points = append(cc[ci].Points, dataset[ri])
				points[ri] = ci

				// Ensure that we always see at least one more iteration after
				// randomly assigning a data point to a cluster
				changes = len(dataset)
			}
		}

		if changes > 0 {
			for _, c := range cc {
				Recenter(c)
			}
		}
		if i == IterationThreshold ||
			changes < int(float64(len(dataset))*deltaThreshold) {
			//m2.Printf("Aborting:itr:%d,chg:%d,thr:%d", i, changes, int(float64(len(dataset))*deltaThreshold))
			break
		}
	}
	//for _, c := range cc {
	//	c.Points = nil
	//}
	if cc[0].Center > cc[1].Center {
		tmp := cc[1]
		cc[1] = cc[0]
		cc[0] = tmp
	}
	return cc
}

func Recenter(c *Cluster) {
	sum := int32(0)
	maxNum := int32(0)
	minNum := int32(10000000)
	for _, d := range c.Points {
		sum += d
		if d > maxNum {
			maxNum = d
		}
		if d < minNum {
			minNum = d
		}
	}
	c.Center = sum / int32(len(c.Points))
	c.Max = maxNum
	c.Min = minNum
}

func Escape(p int32, c *Cluster) bool {
	if p >= c.Min && p <= c.Max {
		return false
	}
	if p > c.Max {
		if p-c.Max > c.Center/10 || p-c.Max > 1800 {
			return true
		}
	} else {
		if c.Min-p > c.Center/10 || c.Min-p > 1800 {
			return true
		}
	}
	return false
}
