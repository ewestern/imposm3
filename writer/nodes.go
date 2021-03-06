package writer

import (
	"sync"

	"github.com/ewestern/imposm3/cache"
	"github.com/ewestern/imposm3/database"
	"github.com/ewestern/imposm3/element"
	geomp "github.com/ewestern/imposm3/geom"
	"github.com/ewestern/imposm3/geom/geos"
	"github.com/ewestern/imposm3/mapping"
	"github.com/ewestern/imposm3/stats"
)

type NodeWriter struct {
	OsmElemWriter
	nodes        chan *element.Node
	pointMatcher mapping.NodeMatcher
}

func NewNodeWriter(
	osmCache *cache.OSMCache,
	nodes chan *element.Node,
	inserter database.Inserter,
	progress *stats.Statistics,
	matcher mapping.NodeMatcher,
	srid int,
) *OsmElemWriter {
	nw := NodeWriter{
		OsmElemWriter: OsmElemWriter{
			osmCache: osmCache,
			progress: progress,
			wg:       &sync.WaitGroup{},
			inserter: inserter,
			srid:     srid,
		},
		pointMatcher: matcher,
		nodes:        nodes,
	}
	nw.OsmElemWriter.writer = &nw
	return &nw.OsmElemWriter
}

func (nw *NodeWriter) loop() {
	geos := geos.NewGeos()
	geos.SetHandleSrid(nw.srid)
	defer geos.Finish()

	for n := range nw.nodes {
		nw.progress.AddNodes(1)
		if matches := nw.pointMatcher.MatchNode(n); len(matches) > 0 {
			nw.NodeToSrid(n)
			if nw.expireor != nil {
				nw.expireor.Expire(n.Long, n.Lat)
			}
			point, err := geomp.Point(geos, *n)
			if err != nil {
				if errl, ok := err.(ErrorLevel); !ok || errl.Level() > 0 {
					log.Warn(err)
				}
				continue
			}

			geom, err := geomp.AsGeomElement(geos, point)
			if err != nil {
				log.Warn(err)
				continue
			}

			if nw.limiter != nil {
				parts, err := nw.limiter.Clip(geom.Geom)
				if err != nil {
					log.Warn(err)
					continue
				}
				if len(parts) >= 1 {
					if err := nw.inserter.InsertPoint(n.OSMElem, geom, matches); err != nil {
						log.Warn(err)
						continue
					}
				}
			} else {
				if err := nw.inserter.InsertPoint(n.OSMElem, geom, matches); err != nil {
					log.Warn(err)
					continue
				}
			}

		}
	}
	nw.wg.Done()
}
