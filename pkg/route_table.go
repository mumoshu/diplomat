package diplomat

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

func (s *RouteTable) AddConditionalRouteToTopic(c RouteCondition) string {
	topics, procs := s.Get(c)
	topic := c.ReceiverName()
	topics = append(topics, topic)
	s.Put(&Route{
		RouteCondition: c,
		Topics:         topics,
		Procedures:     procs,
	})
	return topic
}

func (s *RouteTable) DelConditionalRouteToTopic(c RouteCondition) string {
	topics, procs := s.Get(c)
	topic := c.ReceiverName()
	newTopics := []string{}
	for _, t := range topics {
		if t != topic {
			newTopics = append(topics, t)
		}
	}
	s.Put(&Route{
		RouteCondition: c,
		Topics:         newTopics,
		Procedures:     procs,
	})
	return topic
}

func (s *RouteTable) AddConditionalRouteToProcedure(c RouteCondition) string {
	topics, procs := s.Get(c)
	proc := c.ReceiverName()
	procs = append(procs, proc)
	s.Put(&Route{
		RouteCondition: c,
		Topics:         topics,
		Procedures:     procs,
	})
	return proc
}

func (s *RouteTable) DelConditionalRouteToProcedure(c RouteCondition) string {
	topics, procs := s.Get(c)
	proc := c.ReceiverName()
	newProcs := []string{}
	for _, p := range procs {
		if p != proc {
			newProcs = append(procs, proc)
		}
	}
	s.Put(&Route{
		RouteCondition: c,
		Topics:         topics,
		Procedures:     newProcs,
	})
	return proc
}
