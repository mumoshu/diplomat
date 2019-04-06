package main

func (s *RouteTable) GetRoute(ref RouteConditionRef) *Route {
	partition, ok := s.RoutePartitions[ref.HashValue()]
	if ok {
		return partition.Get(ref.ID())
	}
	return nil
}

func (s *RouteTable) Get(ref RouteConditionRef) (topics, procedures []string) {
	m := s.GetRoute(ref)
	if m == nil {
		return []string{}, []string{}
	}
	return m.Topics, m.Procedures
}

func (s *RouteTable) Put(r *Route) {
	partitionID := r.HashValue()
	partition, ok := s.RoutePartitions[partitionID]
	if !ok {
		routeID := r.ID()
		partition = &RoutesPartition{
			Routes: map[RouteConditionID]*Route{routeID: r},
		}
		s.RoutePartitions[partitionID] = partition
	}
	partition.Put(r)
}

func (s *RouteTable) RouteToTopic(c RouteCondition, topic string) {
	topics, procs := s.Get(c)
	topics = append(topics, topic)
	s.Put(&Route{
		RouteCondition: c,
		Topics: topics,
		Procedures: procs,
	})
}

func (s *RouteTable) RouteToProcedure(c RouteCondition, proc string) {
	topics, procs := s.Get(c)
	procs = append(procs, proc)
	s.Put(&Route{
		RouteCondition: c,
		Topics: topics,
		Procedures: procs,
	})
}

