package expcache

import (
	"testing"
	"time"
	"math/rand"
	"math"
	"sync"
)

//test cannot get non-exist data
func TestEmpty(t *testing.T) {
	c := New()
	defer c.Close()
	if c.Get("Dont Exist") != nil {
		t.Error("Retrieve from empty map not return nil")
	}
}

//test cannot cache data with 0 timeout
func TestZeroTimeout(t *testing.T) {
	c := New()
	defer c.Close()
	_ = "breakpoint"
	c.Put("key",1,0)
	if c.Size() > 0 {
		t.Error("Data with 0 timeout saved into cache, should NOT")
	}
}

func TestEntryTimeout(t *testing.T) {
	c := New()
	defer c.Close()
	c.Put("key",1,200) //cache data for 200 millisecond
	//verify cache size changed
	if c.Size() == 0 {
		t.Fatal("Data with 200 timeout fail to save in cache")
	}
	//verify that i can get cached data
	if vi, ok := c.Get("key").(int); !ok || vi != 1 {
		t.Fatal("Failed to get cache data")
	}
	//verfy that i can still get cached data before timeout
	time.Sleep(100*time.Millisecond)
	if vi, ok := c.Get("key").(int); !ok || vi != 1 {
		t.Fatal("Failed to get cache data before timeout")
	}
	//sleep till cached data timeout, recheck it is gone
	time.Sleep(200*time.Millisecond)
	if vi, ok := c.Get("key").(int); ok && vi == 1 {
		t.Fatal("Still get cache data after timeout")
	}
}

func TestTimeoutOrder(t *testing.T) {
	c := New()
	defer c.Close()
	c.Put("k1",1,250)
	c.Put("k2",2,300)
	c.Put("k3",3,100)
	t.Logf("cache content: %v", c.GetContent())
	//check all data cached successfully
	if c.Size() != 3 {
		t.Fatal("invalid cache size")
	}
	if v1,ok := c.Get("k1").(int); !ok || v1 != 1 {
		t.Fatal("Failed to cache data")
	}
	if v2,ok := c.Get("k2").(int); !ok || v2 != 2 {
		t.Fatal("Failed to cache data")
	}
	if v3,ok := c.Get("k3").(int); !ok || v3 != 3 {
		t.Fatal("Failed to cache data")
	}
	time.Sleep(150*time.Millisecond)
	//check k3 expired
	if v3,ok := c.Get("k3").(int); ok && v3 == 3 {
		t.Fatal("Still get expired data")
	}
	//check k2 still exist
	if v2,ok := c.Get("k2").(int); !ok || v2 != 2 {
		t.Fatal("Failed to cache data")
	}
	//verify Remove() 
	c.Remove("k2")
	t.Logf("cache content: %v", c.GetContent())
	if v2,ok := c.Get("k2").(int); ok && v2 == 2 {
		t.Fatal("Still get removed data")
	}
	//after remove verify other data remain
	if v1,ok := c.Get("k1").(int); !ok || v1 != 1 {
		t.Fatal("Failed to cache data")
	}
}

func TestCacheAcess_Parallel(t *testing.T) {
	c := New()
	var wg sync.WaitGroup
	rand.Seed(time.Now().UnixNano())
	maxTO := 500 //ms
	numTask := 10
	numRun := 100
	for i:=0;i<numTask;i++ {
		wg.Add(1)
		go func(tn int) {
			k := tn*1000
			defer wg.Done()
			for j:=0;j<numRun;j++ {
				to := int64(rand.Float64()*float64(maxTO))
				c.Put(k+j, k+j, to)
			}
			//test dup put
			for j:=0;j<numRun;j++ {
				to := int64(rand.Float64()*float64(maxTO))
				c.Put(k+j, k+j, to)
			}
			to := int64(rand.Float64()*float64(maxTO)/2.0)
			time.Sleep(time.Duration(to))
			expCnt := 0
			for j:=0;j<numRun;j++ {
				if c.Get(k+j) == nil {
					expCnt++
				}
				c.Remove(k+j)
			}
			t.Logf("task %d found %d entries expired", tn, expCnt)
		}(i)
	}
	wg.Wait()
}

func BenchmarkPutGetRemove(b *testing.B) {
	b.ReportAllocs()
	rand.Seed(time.Now().UnixNano())
	maxTO := 500 //ms
	to := int64(rand.Float64()*float64(maxTO))
	c := New()
	for i:=0;i<b.N;i++ {
		c.Put(i, i, to+int64(i))
		c.Get(i)
		c.Remove(i)
	}
}

func BenchmarkPutGetRemoveParallel(b *testing.B) {
	b.ReportAllocs()
	rand.Seed(time.Now().UnixNano())
	maxTO := 500 //ms
	to := int64(rand.Float64()*float64(maxTO))
	c := New()
	b.RunParallel(func (pb *testing.PB) {
		v := int64(rand.Float64()*(math.MaxFloat64))
		for pb.Next() {
			c.Put(v,v,to+v)
			c.Get(v)
			c.Remove(v)
			v++
		}
	})
}
