package team1

import (
	obj "SOMAS2023/internal/common/objects"
	"SOMAS2023/internal/common/physics"
	utils "SOMAS2023/internal/common/utils"
	voting "SOMAS2023/internal/common/voting"
	"fmt"
	"math"

	"github.com/google/uuid"
)

// agent specific parameters
const deviateNegative = -0.2     // trust loss on deviation
const deviatePositive = 0.1      // trust gain on non deviation
const effortScaling = 0.1        // scaling factor for effort, highr it is the more effort chages each round
const fairnessScaling = 0.1      // scaling factor for fairness, higher it is the more fairness changes each round
const leaveThreshold = 0.0       // threshold for leaving
const kickThreshold = 0.0        // threshold for kicking
const trustThreshold = 0.7       // threshold for trusting (need to tune)
const fairnessConstant = 1       // weight of fairness in opinion
const joinThreshold = 0.8        // opinion threshold for joining if not same colour
const leaderThreshold = 0.95     // opinion threshold for becoming leader
const trustconstant = 1          // weight of trust in opinion
const effortConstant = 1         // weight of effort in opinion
const fairnessDifference = 0.5   // modifies how much fairness increases of decreases, higher is more increase, 0.5 is fair
const lowEnergyLevel = 0.3       // energy level below which the agent will try to get a lootbox of the desired colour
const leavingThreshold = 0.3     // how low the agent's vote must be to leave bike
const colorOpinionConstant = 0.2 // how much any agent likes any other of the same colour in the objective function

// Governance decision constants
const democracyOpinonThreshold = 0.5
const democracyReputationThreshold = 0.3
const leadershipOpinionThreshold = 0.7
const leadershipReputationThreshold = 0.5
const dictatorshipOpinionThreshold = 0.9
const dictatorshipReputationThreshold = 0.7

type Opinion struct {
	effort   float64
	trust    float64
	fairness float64
	opinion  float64
}

type Biker1 struct {
	*obj.BaseBiker                       // BaseBiker inherits functions from BaseAgent such as GetID(), GetAllMessages() and UpdateAgentInternalState()
	recentVote     voting.LootboxVoteMap // the agent's most recent vote
	recentDecided  uuid.UUID             // the most recent decision
	dislikeVote    bool                  // whether the agent disliked the most recent vote
	opinions       map[uuid.UUID]Opinion
}

// -------------------SETTERS AND GETTERS-----------------------------
// Returns a list of bikers on the same bike as the agent
// func (bb *Biker1) GetFellowBikers() []obj.IBaseBiker {
// 	gs := bb.GetGameState()
// 	bikeId := bb.GetBike()
// 	return gs.GetMegaBikes()[bikeId].GetAgents()
// }

func (bb *Biker1) GetBikeInstance() obj.IMegaBike {
	gs := bb.GetGameState()
	bikeId := bb.GetBike()
	return gs.GetMegaBikes()[bikeId]
}

func (bb *Biker1) GetLootLocation(id uuid.UUID) utils.Coordinates {
	gs := bb.GetGameState()
	lootboxes := gs.GetLootBoxes()
	lootbox := lootboxes[id]
	return lootbox.GetPosition()
}

// func (bb *Biker1) GetAverageOpinionOfAgent(biker uuid.UUID) float64 {
// 	fellowBikers := bb.GetFellowBikers()
// 	opinionSum := 0.0
// 	for _, agent := range fellowBikers {
// 		opinionSum += agent.QueryReputation(biker)
// 	}
// 	return opinionSum / float64(len(fellowBikers))
// }

// -------------------END OF SETTERS AND GETTERS----------------------
// part 1:
// the biker itself doesn't technically have a location (as it's on the map only when it's on a bike)
// in fact this function is only called when the biker needs to make a decision about the pedaling forces
// func (bb *Biker1) GetLocation() utils.Coordinates {
// 	gs := bb.GetGameState()
// 	bikeId := bb.GetBike()
// 	megaBikes := gs.GetMegaBikes()
// 	return megaBikes[bikeId].GetPosition()
// }

// // Success-Relationship algo for calculating selfishness score
// func calculateSelfishnessScore(success float64, relationship float64) float64 {
// 	difference := math.Abs(success - relationship)
// 	var overallScore float64
// 	if success >= relationship {
// 		overallScore = 0.5 + ((difference) / 2)
// 	} else if relationship > success {
// 		overallScore = 0.5 - ((difference) / 2)
// 	}
// 	return overallScore
// }

// func (bb *Biker1) GetSelfishness(agent obj.IBaseBiker) float64 {
// 	pointSum := bb.GetPoints() + agent.GetPoints()
// 	var relativeSuccess float64
// 	if pointSum == 0 {
// 		relativeSuccess = 0.5
// 	} else {
// 		relativeSuccess = float64((agent.GetPoints() - bb.GetPoints()) / (pointSum)) //-1 to 1
// 		relativeSuccess = (relativeSuccess + 1.0) / 2.0                              //shift to 0 to 1
// 	}
// 	id := agent.GetID()
// 	ourRelationship := bb.opinions[id].opinion
// 	return calculateSelfishnessScore(relativeSuccess, ourRelationship)
// }

// ---------------LOOT ALLOCATION FUNCTIONS------------------
// through this function the agent submits their desired allocation of resources
// in the MVP each agent returns 1 whcih will cause the distribution to be equal across all of them
// func (bb *Biker1) DecideAllocation() voting.IdVoteMap {
// 	fellowBikers := bb.GetFellowBikers()

// 	sumEnergyNeeds := 0.0
// 	helpfulAllocation := make(map[uuid.UUID]float64)
// 	selfishAllocation := make(map[uuid.UUID]float64)

// 	for _, agent := range fellowBikers {
// 		energyNeed := 1.0 - agent.GetEnergyLevel()
// 		helpfulAllocation[agent.GetID()] = energyNeed
// 		selfishAllocation[agent.GetID()] = energyNeed
// 		sumEnergyNeeds = sumEnergyNeeds + energyNeed
// 	}

// 	for agentId, _ := range helpfulAllocation {
// 		helpfulAllocation[agentId] /= sumEnergyNeeds
// 	}

// 	sumEnergyNeeds -= (1.0 - bb.GetEnergyLevel()) // remove our energy need from the sum

// 	for agentId, _ := range selfishAllocation {
// 		if agentId != bb.GetID() {
// 			selfishAllocation[agentId] = (selfishAllocation[agentId] / sumEnergyNeeds) * bb.GetEnergyLevel() //NB assuming energy is 0-1
// 		}
// 	}

// 	//3/4) Look in success vector to see relative success of each agent and calculate selfishness score using suc-rel chart (0-1)
// 	//TI - Around line 350, we have Soma`s pseudocode on agent opinion held in bb.Opinion.opinion, lets assume its normalized between 0-1
// 	selfishnessScore := make(map[uuid.UUID]float64)
// 	runningScore := 0.0

// 	for _, agent := range fellowBikers {
// 		if agent.GetID() != bb.GetID() {
// 			score := bb.GetSelfishness(agent)
// 			id := agent.GetID()
// 			selfishnessScore[id] = score
// 			runningScore = runningScore + selfishnessScore[id]
// 		}
// 	}

// 	selfishnessScore[bb.GetID()] = runningScore / float64((len(fellowBikers) - 1))

// 	//5) Linearly interpolate between selfish and helpful allocations based on selfishness score
// 	distribution := make(map[uuid.UUID]float64)
// 	runningDistribution := 0.0
// 	for _, agent := range fellowBikers {
// 		id := agent.GetID()
// 		Adistribution := (selfishnessScore[id] * selfishAllocation[id]) + ((1.0 - selfishnessScore[id]) * helpfulAllocation[id])
// 		distribution[id] = Adistribution
// 		runningDistribution = runningDistribution + Adistribution
// 	}
// 	for agentId, _ := range distribution {
// 		distribution[agentId] = distribution[agentId] / runningDistribution // Normalise!
// 	}
// 	return distribution
// }

// ---------------END OF LOOT ALLOCATION FUNCTIONS------------------

// ---------------DIRECTION DECISION FUNCTIONS------------------

// Simulates a step of the game, assuming all bikers pedal with the same force as us.
// Returns the distance travelled and the remaining energy
func (bb *Biker1) simulateGameStep(energy float64, velocity float64, force float64) (float64, float64) {
	bikerNum := len(bb.GetFellowBikers())
	totalBikerForce := force * float64(len(bb.GetFellowBikers()))
	totalMass := utils.MassBike + float64(bikerNum)*utils.MassBiker
	acceleration := physics.CalcAcceleration(totalBikerForce, totalMass, velocity)
	distance := velocity + 0.5*acceleration
	energy = energy - force*utils.MovingDepletion
	return distance, energy
}

// Calculates the approximate distance that can be travelled with the given energy
func (bb *Biker1) energyToReachableDistance(energy float64) float64 {
	distance := 0.0
	totalDistance := 0.0
	remainingEnergy := energy
	for remainingEnergy > 0 {
		distance, remainingEnergy = bb.simulateGameStep(remainingEnergy, bb.GetBikeInstance().GetVelocity(), bb.getPedalForce())
		totalDistance = totalDistance + distance
	}
	return totalDistance
}

// Calculates the energy remaining after travelling the given distance
func (bb *Biker1) distanceToEnergy(distance float64, initialEnergy float64) float64 {
	totalDistance := 0.0
	remainingEnergy := initialEnergy
	extraDist := 0.0
	for totalDistance < distance {
		extraDist, remainingEnergy = bb.simulateGameStep(remainingEnergy, bb.GetBikeInstance().GetPhysicalState().Mass, utils.BikerMaxForce*remainingEnergy)
		totalDistance = totalDistance + extraDist
	}

	return remainingEnergy
}

// returns the nearest lootbox with respect to the agent's bike current position
// in the MVP this is used to determine the pedalling forces as all agent will be
// aiming to get to the closest lootbox by default
// func (bb *Biker1) nearestLoot() uuid.UUID {
// 	currLocation := bb.GetLocation()
// 	shortestDist := math.MaxFloat64
// 	var nearestBox uuid.UUID
// 	var currDist float64
// 	for _, loot := range bb.GetGameState().GetLootBoxes() {
// 		x, y := loot.GetPosition().X, loot.GetPosition().Y
// 		currDist = math.Sqrt(math.Pow(currLocation.X-x, 2) + math.Pow(currLocation.Y-y, 2))
// 		if currDist < shortestDist {
// 			nearestBox = loot.GetID()
// 			shortestDist = currDist
// 		}
// 	}
// 	return nearestBox
// }

// Finds the nearest reachable box
func (bb *Biker1) getNearestBox(currLocation utils.Coordinates) uuid.UUID {
	shortestDist := math.MaxFloat64
	//default to nearest lootbox
	nearestBox := uuid.Nilxx
	var currDist float64
	initialized := false
	for id, loot := range bb.GetGameState().GetLootBoxes() {
		if !initialized {
			nearestBox = id
			initialized = true
		}
		lootPos := loot.GetPosition()
		currDist = physics.ComputeDistance(currLocation, lootPos)
		if currDist < shortestDist {
			nearestBox = id
			shortestDist = currDist
		}
	}

	return nearestBox
}

// Finds the nearest lootbox of agent's colour
func (bb *Biker1) nearestLootColour(currLocation utils.Coordinates, agentColour utils.Colour) (uuid.UUID, float64) {
	shortestDist := math.MaxFloat64
	nearestBox := uuid.Nil
	initialized := false
	//default to nearest lootbox
	var currDist float64
	for id, loot := range bb.GetGameState().GetLootBoxes() {
		if !initialized {
			nearestBox = id
			initialized = true
		}
		lootPos := loot.GetPosition()
		currDist = physics.ComputeDistance(currLocation, lootPos)
		if (currDist < shortestDist) && (loot.GetColour() == agentColour) {
			nearestBox = id
			shortestDist = currDist
		}
	}
	return nearestBox, shortestDist
}

func (bb *Biker1) FindReachableBoxNearestToColour(nearestColourBox uuid.UUID) uuid.UUID {
	minDist := math.MaxFloat64
	nearestBox := uuid.Nil
	boxes := bb.GetGameState().GetLootBoxes()
	currBoxLocation := boxes[nearestColourBox].GetPosition()
	ourLocation := bb.GetLocation()
	var currDist float64
	var ourDist float64
	for _, loot := range boxes {
		lootPos := loot.GetPosition()
		currDist = physics.ComputeDistance(currBoxLocation, lootPos)
		ourDist = physics.ComputeDistance(lootPos, ourLocation)
		if ourDist < bb.energyToReachableDistance(bb.GetEnergyLevel()) && currDist < minDist {
			minDist = currDist
			nearestBox = loot.GetID()
		}
	}
	return nearestBox
}

func (bb *Biker1) ProposeDirection() uuid.UUID {
	// get box of our colour
	nearestColourBox, distanceToNearestBox := bb.nearestLootColour(bb.GetLocation(), bb.GetColour())

	// if box of our colour exists
	if nearestColourBox != uuid.Nil {
		reachableDistance := bb.energyToReachableDistance(bb.GetEnergyLevel())
		if distanceToNearestBox < reachableDistance {
			// if reachable, nominate C
			fmt.Printf("agent %v nominated nearest COLOUR %v box %v \n", bb.GetColour(), bb.GetGameState().GetLootBoxes()[nearestColourBox].GetColour(), nearestColourBox)
			return nearestColourBox
		} else {
			nearestBox := bb.FindReachableBoxNearestToColour(nearestColourBox)
			if nearestBox != uuid.Nil {
				fmt.Printf("agent %v nominated %v box nearest to COLOUR %v %v \n", bb.GetColour(), bb.GetGameState().GetLootBoxes()[nearestBox].GetColour(), bb.GetColour(), nearestBox)
				return nearestBox
			}
		}
	}

	// assumed that box always exists
	nearestBox := bb.getNearestBox(bb.GetLocation())
	fmt.Printf("agent %v nominatxed nearest %v box %v \n", bb.GetColour(), bb.GetGameState().GetLootBoxes()[nearestBox].GetColour(), nearestBox)
	return nearestBox
}

func (bb *Biker1) distanceToBox(box uuid.UUID) float64 {
	currLocation := bb.GetLocation()
	boxPos := bb.GetGameState().GetLootBoxes()[box].GetPosition()
	currDist := physics.ComputeDistance(currLocation, boxPos)
	return currDist
}

func (bb *Biker1) findRemainingEnergyAfterReachingBox(box uuid.UUID) float64 {
	dist := physics.ComputeDistance(bb.GetLocation(), bb.GetGameState().GetLootBoxes()[box].GetPosition())
	remainingEnergy := bb.distanceToEnergy(dist, bb.GetEnergyLevel())
	return remainingEnergy
}

// Checks whether a box of the desired colour is within our reachable distance from a given box
func (bb *Biker1) checkBoxNearColour(box uuid.UUID, energy float64) uuid.UUID {
	lootBoxes := bb.GetGameState().GetLootBoxes()
	boxPos := lootBoxes[box].GetPosition()
	var currDist float64
	for _, loot := range lootBoxes {
		lootPos := loot.GetPosition()
		currDist = physics.ComputeDistance(boxPos, lootPos)
		if (currDist <= bb.energyToReachableDistance(energy)) && (loot.GetColour() == bb.GetColour()) {
			return loot.GetID()
		}
	}
	return uuid.Nil
}

func (bb *Biker1) calculateCubeScoreForAgent(agent obj.IBaseBiker) float64 {
	agentPoints := float64(agent.GetPoints())
	ourPoints := float64(bb.GetPoints())
	agentOpinion := bb.opinions[agent.GetID()].opinion
	agentEnergy := agent.GetEnergyLevel()
	ourEnergy := bb.GetEnergyLevel()

	maxPoints := ourPoints + agentPoints // TODO use maxPoints
	relPoints := 0.0
	if maxPoints == 0 {
		relPoints = 0.5
	} else {
		relPoints = (((agentPoints - ourPoints) / (maxPoints + 0.00001)) + 1) / 2
	}

	relEnergy := ((agentEnergy - ourEnergy) + 1) / 2

	// Check Spec for cube explanation
	cubeScore := -0.3*relEnergy - 0.2*relPoints + 0.5*agentOpinion + 0.5
	return cubeScore
}

func (bb *Biker1) calcEnergyScore(destBoxID uuid.UUID, curBoxID uuid.UUID, curEnergy float64) float64 {
	boxes := bb.GetGameState().GetLootBoxes()
	destBox := boxes[destBoxID]
	curBox := boxes[curBoxID]

	destBoxLoc := destBox.GetPosition()
	curBoxLoc := curBox.GetPosition()

	dist := physics.ComputeDistance(destBoxLoc, curBoxLoc)

	energyToTravel := bb.distanceToEnergy(dist, curEnergy)

	return energyToTravel / curEnergy
}

func (bb *Biker1) FinalDirectionVote(proposals map[uuid.UUID]uuid.UUID) voting.LootboxVoteMap {
	votes := make(voting.LootboxVoteMap)
	maxDist := bb.energyToReachableDistance(bb.GetEnergyLevel())

	// make map of proposal:numberofnoms and find maximum number of votes
	// make map of proposal:proposers
	proposalNoOfNoms := make(map[uuid.UUID]int)
	maxVotes := 1
	curVotes := 1
	for _, proposal := range proposals {
		// initialise final votes as 0.
		votes[proposal] = 0.0

		// add 1 to number of noms for each proposal
		_, exists := proposalNoOfNoms[proposal]
		if !exists {
			proposalNoOfNoms[proposal] = 1
		} else {
			proposalNoOfNoms[proposal] += 1
			if proposal != proposals[bb.GetID()] {
				curVotes = proposalNoOfNoms[proposal]
				if curVotes > maxVotes {
					maxVotes = curVotes
				}
			}
		}
	}
	fmt.Printf("Max Votes: %v\n", maxVotes)
	// if our proposal has majority noms, vote for it
	if proposalNoOfNoms[proposals[bb.GetID()]] > maxVotes {
		votes[proposals[bb.GetID()]] = 1
		fmt.Printf("%v votes for its own nomination %v\n", bb.GetColour(), votes)
		return votes
	}

	// for every nominated box (D)
	for proposer, proposal := range proposals {
		if maxDist < bb.distanceToBox(proposal) {
			// if it is not reachable, ignore
			continue
		}

		// calculate energy left if travelled to D
		remainingEnergy := bb.findRemainingEnergyAfterReachingBox(proposal)

		// check if our colour is reachable with this remaining energy from D
		nearColourBox := bb.checkBoxNearColour(proposal, remainingEnergy)
		// if no boxes of our colour are reachable from this box, assign vote of 0 and continue
		if nearColourBox == uuid.Nil {
			votes[proposal] = 0.0
			continue
		}

		// add this proposer's cube score to the votes map for this box
		votes[proposal] += bb.calculateCubeScoreForAgent(bb.GetAgentFromId(proposer))

		// calculate energy score to add
		energyScore := bb.calcEnergyScore(nearColourBox, proposal, remainingEnergy)

		// add (1/noms_for_D) * energyScore because we add this noms_for_D times
		votes[proposal] += energyScore / float64(proposalNoOfNoms[proposal])
	}

	// if all nominations have score 0, assign 1 to box we nominated
	allVotesZero := true
	for _, value := range votes {
		if value != 0 {
			allVotesZero = false
			break
		}
	}
	if allVotesZero {
		votes[proposals[bb.GetID()]] = 1.
	}

	// normalise values
	sum := 0.0
	for _, value := range votes {
		sum += value
	}
	for key := range votes {
		votes[key] /= sum
	}
	fmt.Printf("%v normalised votes pre-selection: %v\n", bb.GetColour(), votes)

	maxVote := 0.0
	var finalProposal uuid.UUID
	for proposal, value := range votes {
		if value >= maxVote {
			maxVote = value
			finalProposal = proposal
		}
		votes[proposal] = 0.0
	}
	votes[finalProposal] = 1.
	fmt.Printf("%v normalised votes post-selection: %v\n", bb.GetColour(), votes)

	return votes
}

// this function will contain the agent's strategy on deciding which direction to go to
// the default implementation returns an equal distribution over all options
// this will also be tried as returning a rank of options
// func (bb *Biker1) OldFinalDirectionVote(proposals map[uuid.UUID]uuid.UUID) voting.LootboxVoteMap {

// 	votes := make(voting.LootboxVoteMap)
// 	maxDist := bb.energyToReachableDistance(bb.GetEnergyLevel())

// 	proposalVotes := make(map[uuid.UUID]int)

// 	maxVotes := 1
// 	curVotes := 1
// 	fmt.Printf("proposals: %v\n", proposals)
// 	for _, proposal := range proposals {
// 		_, ok := proposalVotes[proposal]
// 		if !ok {
// 			proposalVotes[proposal] = 1
// 		} else {
// 			proposalVotes[proposal] += 1
// 			if proposal != proposals[bb.GetID()] {
// 				curVotes = proposalVotes[proposal]
// 				if curVotes > maxVotes {
// 					maxVotes = curVotes
// 				}
// 			}
// 		}
// 	}
// 	distToBoxMap := make(map[uuid.UUID]float64)
// 	for _, proposal := range proposals {
// 		distToBoxMap[proposal] = bb.distanceToBox(proposal)
// 		if distToBoxMap[proposal] <= maxDist { //if reachable
// 			// if box is our colour and number of proposals is majority, make it 1, rest 0, return
// 			if bb.GetGameState().GetLootBoxes()[proposal].GetColour() == bb.GetColour() {
// 				if proposalVotes[proposal] >= maxVotes { // to c
// 					for _, proposal1 := range proposals {
// 						if proposal1 == proposal {
// 							votes[proposal1] = 1
// 						} else {
// 							votes[proposal1] = 0
// 						}
// 					}
// 					break
// 				} else {
// 					votes[proposal] = float64(proposalVotes[proposal])
// 				}
// 			}
// 			// calculate energy left if we went here
// 			remainingEnergy := bb.findRemainingEnergyAfterReachingBox(proposal)
// 			// find nearest reachable boxes from current coordinate
// 			isColourNear := bb.checkBoxNearColour(proposal, remainingEnergy)
// 			// assign score of number of votes for this box if our colour is nearby
// 			if isColourNear {
// 				votes[proposal] = float64(proposalVotes[proposal])
// 			} else {
// 				votes[proposal] = 0.0
// 			}
// 		} else {
// 			votes[proposal] = 0.0
// 		}
// 	}
// 	fmt.Printf("Our votes that are not normalised: %v\n", votes)

// 	// Check if all votes are 0
// 	allVotesZero := true
// 	for _, value := range votes {
// 		if value != 0 {
// 			allVotesZero = false
// 			break
// 		}
// 	}

// 	// If all votes are 0, nominate the nearest box
// 	// Maybe nominate our box?
// 	if allVotesZero {
// 		minDist := math.MaxFloat64
// 		var nearestBox uuid.UUID
// 		for _, proposal := range proposals {
// 			if distToBoxMap[proposal] < minDist {
// 				minDist = distToBoxMap[proposal]
// 				nearestBox = proposal
// 			}
// 		}
// 		votes[nearestBox] = 1
// 		fmt.Printf("our votes are which are all 0: %v\n", votes)
// 		return votes
// 	}

// 	// Normalize the values in votes so that the values sum to 1
// 	sum := 0.0
// 	for _, value := range votes {
// 		sum += value
// 	}
// 	for key := range votes {
// 		votes[key] /= sum
// 	}
// 	fmt.Printf("our votes are: normalised %v\n", votes)

// 	return votes
// }

// -----------------END OF DIRECTION DECISION FUNCTIONS------------------

// func (bb *Biker1) DecideAction() obj.BikerAction {
// 	// fellowBikers := bb.GetFellowBikers()
// 	// avg_opinion := 1.0
// 	// for _, agent := range fellowBikers {
// 	// 	avg_opinion = avg_opinion + bb.opinions[agent.GetID()].opinion
// 	// }
// 	// if (avg_opinion < leaveThreshold) || bb.dislikeVote {
// 	// 	bb.dislikeVote = false
// 	// 	return 1
// 	// } else {
// 	// 	return 0
// 	// }
// 	return 0
// }

// // -----------------PEDALLING FORCE FUNCTIONS------------------
func (bb *Biker1) getPedalForce() float64 {
	//can be made more complex
	return utils.BikerMaxForce * 0.3 ///bb.GetEnergyLevel()
}

// // determine the forces (pedalling, breaking and turning)
// // in the MVP the pedalling force will be 1, the breaking 0 and the tunring is determined by the
// // location of the nearest lootboX
// // the function is passed in the id of the voted lootbox, for now ignored
func (bb *Biker1) DecideForce(direction uuid.UUID) {
	fmt.Printf("\nscore: %v \n", bb.GetPoints())
	if bb.recentVote != nil {
		if bb.recentVote[direction] < leavingThreshold {
			bb.dislikeVote = true
		} else {
			bb.dislikeVote = false
		}
	}

	//agent doesn't rebel, just decides to leave next round if dislike vote
	lootBoxes := bb.GetGameState().GetLootBoxes()
	currLocation := bb.GetLocation()
	if len(lootBoxes) > 0 {
		targetPos := lootBoxes[direction].GetPosition()
		deltaX := targetPos.X - currLocation.X
		deltaY := targetPos.Y - currLocation.Y
		angle := math.Atan2(deltaY, deltaX)
		normalisedAngle := angle / math.Pi

		turningDecision := utils.TurningDecision{
			SteerBike:     true,
			SteeringForce: normalisedAngle - bb.GetBikeInstance().GetOrientation(),
		}
		boxForces := utils.Forces{
			Pedal:   bb.getPedalForce(),
			Brake:   0.0,
			Turning: turningDecision,
		}
		bb.SetForces(boxForces)
	} else { //shouldnt happen, but would just run from audi
		audiPos := bb.GetGameState().GetAudi().GetPosition()
		deltaX := audiPos.X - currLocation.X
		deltaY := audiPos.Y - currLocation.Y
		// Steer in opposite direction to audi
		angle := math.Atan2(-deltaY, -deltaX)
		normalisedAngle := angle / math.Pi
		turningDecision := utils.TurningDecision{
			SteerBike:     true,
			SteeringForce: normalisedAngle - bb.GetBikeInstance().GetOrientation(),
		}

		escapeAudiForces := utils.Forces{
			Pedal:   bb.getPedalForce(),
			Brake:   0.0,
			Turning: turningDecision,
		}
		bb.SetForces(escapeAudiForces)
	}
}

// // -----------------END OF PEDALLING FORCE FUNCTIONS------------------

// // -----------------OPINION FUNCTIONS------------------

// func (bb *Biker1) UpdateEffort(agentID uuid.UUID) {
// 	agent := bb.GetAgentFromId(agentID)
// 	fellowBikers := bb.GetFellowBikers()
// 	totalPedalForce := 0.0
// 	for _, agent := range fellowBikers {
// 		totalPedalForce = totalPedalForce + agent.GetForces().Pedal
// 	}
// 	avgForce := totalPedalForce / float64(len(fellowBikers))
// 	//effort expectation is scaled by their energy level -- should it be? (*agent.GetEnergyLevel())
// 	finalEffort := bb.opinions[agent.GetID()].effort + (agent.GetForces().Pedal-avgForce)*effortScaling

// 	if finalEffort > 1 {
// 		finalEffort = 1
// 	}
// 	if finalEffort < 0 {
// 		finalEffort = 0
// 	}
// 	newOpinion := Opinion{
// 		effort:   finalEffort,
// 		fairness: bb.opinions[agentID].fairness,
// 		trust:    bb.opinions[agentID].trust,
// 		opinion:  bb.opinions[agentID].opinion,
// 	}
// 	bb.opinions[agent.GetID()] = newOpinion
// }

// func (bb *Biker1) UpdateTrust(agentID uuid.UUID) {
// 	id := agentID
// 	agent := bb.GetAgentFromId(agentID)
// 	finalTrust := 0.5
// 	if agent.GetForces().Turning.SteeringForce == bb.GetForces().Turning.SteeringForce {
// 		finalTrust = bb.opinions[id].trust + deviatePositive
// 		if finalTrust > 1 {
// 			finalTrust = 1
// 		}
// 	} else {
// 		finalTrust := bb.opinions[id].trust + deviateNegative
// 		if finalTrust < 0 {
// 			finalTrust = 0
// 		}
// 	}
// 	newOpinion := Opinion{
// 		effort:   bb.opinions[id].effort,
// 		fairness: bb.opinions[id].fairness,
// 		trust:    finalTrust,
// 		opinion:  bb.opinions[id].opinion,
// 	}
// 	bb.opinions[id] = newOpinion
// }

// // func (bb *Biker1) UpdateFairness(agent obj.IBaseBiker) {
// // 	difference := 0.0
// // 	agentVote := agent.DecideAllocation()
// // 	fairVote := bb.DecideAllocation()
// // 	//If anyone has a better solution fo this please do it, couldn't find a better way to substract two maps in go
// // 	for i, theirVote := range agentVote {
// // 		for j, ourVote := range fairVote {
// // 			if i == j {
// // 				difference = difference + math.Abs(ourVote - theirVote)
// // 			}
// // 		}
// // 	}
// // 	finalFairness := bb.opinions[agent.GetID()].fairness + (fairnessDifference - difference/2)*fairnessScaling

// // 	if finalFairness > 1 {
// // 		finalFairness = 1
// // 	}
// // 	if finalFairness < 0 {
// // 		finalFairness = 0
// // 	}
// // 	agentID := agent.GetID()
// // 	newOpinion := Opinion{
// // 		effort:   bb.opinions[agentID].effort,
// // 		fairness: finalFairness,
// // 		trust:    bb.opinions[agentID].trust,
// // 		opinion:  bb.opinions[agentID].opinion,
// // 	}
// // 	bb.opinions[agent.GetID()] = newOpinion
// // }

// // how well does agent 1 like agent 2 according to objective metrics
// func (bb *Biker1) GetObjectiveOpinion(id1 uuid.UUID, id2 uuid.UUID) float64 {
// 	agent1 := bb.GetAgentFromId(id1)
// 	agent2 := bb.GetAgentFromId(id2)
// 	objOpinion := 0.0
// 	if agent1.GetColour() == agent2.GetColour() {
// 		objOpinion = objOpinion + colorOpinionConstant
// 	}
// 	objOpinion = objOpinion + (agent1.GetEnergyLevel() - agent2.GetEnergyLevel())
// 	gs := bb.GetGameState()
// 	megabikes := gs.GetMegaBikes()
// 	maxpoints := 0
// 	for _, bike := range megabikes {
// 		for _, agent := range bike.GetAgents() {
// 			if agent.GetPoints() > maxpoints {
// 				maxpoints = agent.GetPoints()
// 			}
// 		}
// 	}
// 	objOpinion = objOpinion + float64((agent1.GetPoints()-agent2.GetPoints())/maxpoints)
// 	objOpinion = math.Abs(objOpinion / (2.0 + colorOpinionConstant)) //normalise to 0-1
// 	return objOpinion
// }

// func (bb *Biker1) UpdateOpinions() {
// 	fellowBikers := bb.GetFellowBikers()
// 	for _, agent := range fellowBikers {
// 		id := agent.GetID()
// 		_, ok := bb.opinions[agent.GetID()]

// 		if !ok {
// 			agentId := agent.GetID()
// 			//if we have no data on an agent, initialise to neutral
// 			newOpinion := Opinion{
// 				effort:   0.5,
// 				trust:    0.5,
// 				fairness: 0.5,
// 				opinion:  0.5,
// 			}
// 			bb.opinions[agentId] = newOpinion
// 		}
// 		bb.UpdateTrust(id)
// 		bb.UpdateEffort(id)
// 		//bb.UpdateFairness(agent)
// 		bb.UpdateOpinion(id, 1.0)
// 	}
// }

// func (bb *Biker1) UpdateOpinion(id uuid.UUID, multiplier float64) {
// 	//Sorry no youre right, keep it, silly me
// 	newOpinion := Opinion{
// 		effort:   bb.opinions[id].effort,
// 		trust:    bb.opinions[id].trust,
// 		fairness: bb.opinions[id].fairness,
// 		opinion:  ((bb.opinions[id].trust*trustconstant+bb.opinions[id].effort*effortConstant+bb.opinions[id].fairness*fairnessConstant)/trustconstant + effortConstant + fairnessConstant) * multiplier,
// 	}
// 	if newOpinion.opinion > 1 {
// 		newOpinion.opinion = 1
// 	} else if newOpinion.opinion < 0 {
// 		newOpinion.opinion = 0
// 	}
// 	bb.opinions[id] = newOpinion

// }

// func (bb *Biker1) ourReputation() float64 {
// 	fellowBikers := bb.GetFellowBikers()
// 	repuation := 0.0
// 	for _, agent := range fellowBikers {
// 		repuation = repuation + bb.GetObjectiveOpinion(bb.GetID(), agent.GetID())

// 	}
// 	repuation = repuation / float64(len(fellowBikers))
// 	return repuation
// }

// // ----------------END OF OPINION FUNCTIONS--------------

// // -----------------MESSAGING FUNCTIONS------------------

// // Handle a message received from anyone, ensuring they are trustworthy and come from the right place (e.g. our bike)
// func (bb *Biker1) VerifySender(sender obj.IBaseBiker) bool {
// 	// check if sender is on our bike
// 	if sender.GetBike() == bb.GetBike() {
// 		// check if sender is trustworthy
// 		if bb.opinions[sender.GetID()].trust > trustThreshold {
// 			return true
// 		}
// 	}
// 	return false
// }

// // Agent receives a who to kick off message
// func (bb *Biker1) HandleKickOffMessage(msg obj.KickOffAgentMessage) {
// 	sender := msg.GetSender()
// 	verified := bb.VerifySender(sender)
// 	if verified {
// 		// slightly penalise view of person who sent message
// 		penalty := 0.9
// 		bb.UpdateOpinion(sender.GetID(), penalty)
// 	}

// }

// // Agent receives a reputation of another agent
// func (bb *Biker1) HandleReputationMessage(msg obj.ReputationOfAgentMessage) {
// 	sender := msg.GetSender()
// 	verified := bb.VerifySender(sender)
// 	if verified {
// 		// TODO: SOME FORMULA TO UPDATE OPINION BASED ON REPUTATION given
// 	}
// }

// // Agent receives a message from another agent to join
// func (bb *Biker1) HandleJoiningMessage(msg obj.JoiningAgentMessage) {
// 	sender := msg.GetSender()
// 	// check if sender is trustworthy
// 	if bb.opinions[sender.GetID()].trust > trustThreshold {
// 		// TODO: some update on opinon maybe???
// 	}

// }

// // Agent receives a message from another agent say what lootbox they want to go to
// func (bb *Biker1) HandleLootboxMessage(msg obj.LootboxMessage) {
// 	sender := msg.GetSender()
// 	verified := bb.VerifySender(sender)
// 	if verified {
// 		// TODO: some update on lootbox decision maybe??
// 	}
// }

// // Agent receives a message from another agent saying what Governance they want
// func (bb *Biker1) HandleGovernanceMessage(msg obj.GovernanceMessage) {
// 	sender := msg.GetSender()
// 	verified := bb.VerifySender(sender)
// 	if verified {
// 		// TODO: some update on governance decision maybe??
// 	}
// }

// func (bb *Biker1) GetTrustedRecepients() []obj.IBaseBiker {
// 	fellowBikers := bb.GetFellowBikers()
// 	var trustedRecepients []obj.IBaseBiker
// 	for _, agent := range fellowBikers {
// 		if bb.opinions[agent.GetID()].trust > trustThreshold {
// 			trustedRecepients = append(trustedRecepients, agent)
// 		}
// 	}
// 	return trustedRecepients
// }

// // CREATING MESSAGES
// func (bb *Biker1) CreateKickOffMessage() obj.KickOffAgentMessage {
// 	// Receipients = fellowBikers
// 	agentToKick := bb.lowestOpinionKick()
// 	var kickDecision bool
// 	// send kick off message if we have a low opinion of someone
// 	if agentToKick != uuid.Nil {
// 		kickDecision = true
// 	} else {
// 		kickDecision = false
// 	}

// 	return obj.KickOffAgentMessage{
// 		BaseMessage: messaging.CreateMessage[obj.IBaseBiker](bb, bb.GetTrustedRecepients()),
// 		AgentId:     agentToKick,
// 		KickOff:     kickDecision,
// 	}
// }

// func (bb *Biker1) CreateReputationMessage() obj.ReputationOfAgentMessage {
// 	// Tell the truth (for now)
// 	// TODO: receipients = fellowBikers that we trust?
// 	return obj.ReputationOfAgentMessage{
// 		BaseMessage: messaging.CreateMessage[obj.IBaseBiker](bb, bb.GetTrustedRecepients()),
// 		AgentId:     uuid.Nil,
// 		Reputation:  1.0,
// 	}
// }

// func (bb *Biker1) CreateJoiningMessage() obj.JoiningAgentMessage {
// 	// Tell the truth (for now)
// 	// receipients = fellowBikers
// 	biketoJoin := bb.ChangeBike()
// 	gs := bb.GetGameState()
// 	joiningBike := gs.GetMegaBikes()[biketoJoin]
// 	return obj.JoiningAgentMessage{
// 		BaseMessage: messaging.CreateMessage[obj.IBaseBiker](bb, joiningBike.GetAgents()),
// 		AgentId:     bb.GetID(),
// 		BikeId:      biketoJoin,
// 	}
// }
// func (bb *Biker1) CreateLootboxMessage() obj.LootboxMessage {
// 	// Tell the truth (for now)
// 	// receipients = fellowBikers
// 	chosenLootbox := bb.ProposeDirection()
// 	return obj.LootboxMessage{
// 		BaseMessage: messaging.CreateMessage[obj.IBaseBiker](bb, bb.GetTrustedRecepients()),
// 		LootboxId:   chosenLootbox,
// 	}
// }

// func (bb *Biker1) CreateGoverenceMessage() obj.GovernanceMessage {
// 	// Tell the truth (using same logic as deciding governance for voting) (for now)
// 	// receipients = fellowBikers
// 	chosenGovernance := bb.DecideGovernace()
// 	return obj.GovernanceMessage{
// 		BaseMessage:  messaging.CreateMessage[obj.IBaseBiker](bb, bb.GetTrustedRecepients()),
// 		BikeId:       bb.GetBike(),
// 		GovernanceId: chosenGovernance,
// 	}
// }

// // Agent sending messages to other agents
// func (bb *Biker1) GetAllMessages([]obj.IBaseBiker) []messaging.IMessage[obj.IBaseBiker] {
// 	var sendKickMessage, sendReputationMessage, sendJoiningMessage, sendLootboxMessage, sendGovernanceMessage bool

// 	// TODO: add logic to decide which messages to send and when

// 	var messageList []messaging.IMessage[obj.IBaseBiker]
// 	if sendKickMessage {
// 		messageList = append(messageList, bb.CreateKickOffMessage())
// 	}
// 	if sendReputationMessage {
// 		messageList = append(messageList, bb.CreateReputationMessage())
// 	}
// 	if sendJoiningMessage {
// 		messageList = append(messageList, bb.CreateJoiningMessage())
// 	}
// 	if sendLootboxMessage {
// 		messageList = append(messageList, bb.CreateLootboxMessage())

// 	}
// 	if sendGovernanceMessage {
// 		messageList = append(messageList, bb.CreateGoverenceMessage())

// 	}
// 	return messageList
// }

// -----------------END MESSAGING FUNCTIONS------------------

// ----------------CHANGE BIKE FUNCTIONS-----------------
// define a sorter for bikes -> used to change bikes
// type BikeSorter struct {
// 	bikes []bikeDistance
// 	by    func(b1, b2 *bikeDistance) bool
// }

// func (sorter *BikeSorter) Len() int {
// 	return len(sorter.bikes)
// }
// func (sorter *BikeSorter) Swap(i, j int) {
// 	sorter.bikes[i], sorter.bikes[j] = sorter.bikes[j], sorter.bikes[i]
// }
// func (sorter *BikeSorter) Less(i, j int) bool {
// 	return sorter.by(&sorter.bikes[i], &sorter.bikes[j])
// }

// type bikeDistance struct {
// 	bikeID   uuid.UUID
// 	bike     obj.IMegaBike
// 	distance float64
// }
// type By func(b1, b2 *bikeDistance) bool

// func (by By) Sort(bikes []bikeDistance) {
// 	ps := &BikeSorter{
// 		bikes: bikes,
// 		by:    by,
// 	}
// 	sort.Sort(ps)
// }

// Calculate how far we can jump for another bike -> based on energy level
// func (bb *Biker1) GetMaxJumpDistance() float64 {
// 	//default to half grid size
// 	//TODO implement this
// 	return utils.GridHeight / 2
// }
// func (bb *Biker1) BikeOurColour(bike obj.IMegaBike) bool {
// 	matchCounter := 0
// 	totalAgents := len(bike.GetAgents())
// 	for _, agent := range bike.GetAgents() {
// 		bbColour := bb.GetColour()
// 		agentColour := agent.GetColour()
// 		if agentColour != bbColour {
// 			matchCounter++
// 		}
// 	}
// 	if matchCounter > totalAgents/2 {
// 		return true
// 	} else {
// 		return false
// 	}
// }

// decide which bike to go to
// func (bb *Biker1) ChangeBike() uuid.UUID {
// 	distance := func(b1, b2 *bikeDistance) bool {
// 		return b1.distance < b2.distance
// 	}
// 	gs := bb.GetGameState()
// 	allBikes := gs.GetMegaBikes()
// 	var bikeDistances []bikeDistance
// 	for id, bike := range allBikes {
// 		if len(bike.GetAgents()) < 8 {
// 			dist := physics.ComputeDistance(bb.GetLocation(), bike.GetPosition())
// 			if dist < bb.GetMaxJumpDistance() {
// 				bikeDistances = append(bikeDistances, bikeDistance{
// 					bikeID:   id,
// 					bike:     bike,
// 					distance: dist,
// 				})
// 			}

// 		}
// 	}

// 	By(distance).Sort(bikeDistances)
// 	for _, bike := range bikeDistances {
// 		if bb.BikeOurColour(bike.bike) {
// 			return bike.bikeID
// 		}
// 	}
// 	return bikeDistances[0].bikeID
// }

// -------------------END OF CHANGE BIKE FUNCTIONS----------------------
func (bb *Biker1) GetAgentFromId(agentId uuid.UUID) obj.IBaseBiker {
	gs := bb.GetGameState()
	bikes := gs.GetMegaBikes()
	for bikeId := range bikes {
		for _, agents := range gs.GetMegaBikes()[bikeId].GetAgents() {
			if agents.GetID() == agentId {
				agent := agents
				return agent
			}
		}
	}
	return bb
}

// -------------------BIKER ACCEPTANCE FUNCTIONS------------------------
// an agent will have to rank the agents that are trying to join and that they will try to
// func (bb *Biker1) DecideJoining(pendingAgents []uuid.UUID) map[uuid.UUID]bool {
// 	//gs.GetMegaBikes()[bikeId].GetAgents()

// 	decision := make(map[uuid.UUID]bool)

// 	for _, agentId := range pendingAgents {
// 		//TODO FIX
// 		agent := bb.GetAgentFromId(agentId)

// 		bbColour := bb.GetColour()
// 		agentColour := agent.GetColour()
// 		if agentColour == bbColour {
// 			decision[agentId] = true
// 			sameColourReward := 1.05
// 			bb.UpdateOpinion(agentId, sameColourReward)
// 		} else {
// 			if bb.opinions[agentId].opinion > joinThreshold {
// 				decision[agentId] = true
// 				// penalise for accepting them without same colour
// 				penalty := 0.9
// 				bb.UpdateOpinion(agentId, penalty)
// 			}
// 			decision[agentId] = false
// 		}
// 	}
// 	return decision
// }

// func (bb *Biker1) lowestOpinionKick() uuid.UUID {
// 	fellowBikers := bb.GetFellowBikers()
// 	lowestOpinion := kickThreshold
// 	var worstAgent uuid.UUID
// 	for _, agent := range fellowBikers {
// 		if bb.opinions[agent.GetID()].opinion < lowestOpinion {
// 			lowestOpinion = bb.opinions[agent.GetID()].opinion
// 			worstAgent = agent.GetID()
// 		}
// 	}
// 	// if we want to kick someone based on our threshold, return their id, else return nil
// 	if lowestOpinion < kickThreshold {
// 		return worstAgent
// 	}
// 	return uuid.Nil
// }

// func (bb *Biker1) DecideKick(agent uuid.UUID) int {
// 	if bb.opinions[agent].opinion < kickThreshold {
// 		return 1
// 	}
// 	return 0
// }

// func (bb *Biker1) VoteForKickout() map[uuid.UUID]int {
// 	voteResults := make(map[uuid.UUID]int)
// 	fellowBikers := bb.GetFellowBikers()
// 	for _, agent := range fellowBikers {
// 		agentID := agent.GetID()
// 		if agentID != bb.GetID() {
// 			// random votes to other agents
// 			voteResults[agentID] = bb.DecideKick(agentID)
// 		}
// 	}
// 	return voteResults
// }

//--------------------END OF BIKER ACCEPTANCE FUNCTIONS-------------------

// -------------------GOVERMENT CHOICE FUNCTIONS--------------------------

// Not implemented on Server yet so this is just a placeholder
// func (bb *Biker1) DecideGovernace() int {
// 	if bb.DecideDictatorship() {
// 		return 2
// 	} else if bb.DecideLeadership() {
// 		return 1
// 	} else {
// 		// Democracy
// 		return 0
// 	}
// }

// Might be unnecesary as this is the default goverment choice for us
// func (bb *Biker1) DecideDemocracy() bool {
// 	fellowBikers := bb.GetFellowBikers()
// 	totalOpinion := 0.0
// 	reputation := bb.ourReputation()
// 	for _, agent := range fellowBikers {
// 		opinion, ok := bb.opinions[agent.GetID()]
// 		if ok {
// 			totalOpinion = totalOpinion + opinion.opinion
// 		}
// 	}
// 	normOpinion := totalOpinion / float64(len(fellowBikers))
// 	if (normOpinion > democracyOpinonThreshold) || (reputation > democracyReputationThreshold) {
// 		return true
// 	} else {
// 		return false
// 	}
// }

// func (bb *Biker1) DecideLeadership() bool {
// 	fellowBikers := bb.GetFellowBikers()
// 	totalOpinion := 0.0
// 	reputation := bb.ourReputation()
// 	for _, agent := range fellowBikers {
// 		opinion, ok := bb.opinions[agent.GetID()]
// 		if ok {
// 			totalOpinion = totalOpinion + opinion.opinion
// 		}
// 	}
// 	normOpinion := totalOpinion / float64(len(fellowBikers))
// 	if (normOpinion > leadershipOpinionThreshold) || (reputation > leadershipReputationThreshold) {
// 		return true
// 	} else {
// 		return false
// 	}
// }

// func (bb *Biker1) DecideDictatorship() bool {
// 	fellowBikers := bb.GetFellowBikers()
// 	totalOpinion := 0.0
// 	reputation := bb.ourReputation()
// 	for _, agent := range fellowBikers {
// 		opinion, ok := bb.opinions[agent.GetID()]
// 		if ok {
// 			totalOpinion = totalOpinion + opinion.opinion
// 		}
// 	}
// 	normOpinion := totalOpinion / float64(len(fellowBikers))
// 	if (normOpinion > dictatorshipOpinionThreshold) || (reputation > dictatorshipReputationThreshold) {
// 		return true
// 	} else {
// 		return false
// 	}
// }

// ----------------------LEADER/DICTATOR VOTING FUNCTIONS------------------
// func (bb *Biker1) VoteLeader() voting.IdVoteMap {

// 	votes := make(voting.IdVoteMap)
// 	fellowBikers := bb.GetFellowBikers()

// 	maxOpinion := 0.0
// 	leaderVote := bb.GetID()
// 	for _, agent := range fellowBikers {
// 		votes[agent.GetID()] = 0.0
// 		avgOp := bb.GetAverageOpinionOfAgent(agent.GetID())
// 		if agent.GetID() != bb.GetID() {
// 			val, ok := bb.opinions[agent.GetID()]
// 			if ok {
// 				avgOp = (avgOp + val.opinion) / 2
// 			}
// 		}
// 		if avgOp > maxOpinion {
// 			maxOpinion = avgOp
// 			leaderVote = agent.GetID()
// 		}
// 	}
// 	votes[leaderVote] = 1.0
// 	return votes
// }

// func (bb *Biker1) VoteDictator() voting.IdVoteMap {
// 	votes := make(voting.IdVoteMap)
// 	fellowBikers := bb.GetFellowBikers()

// 	maxOpinion := 0.0
// 	leaderVote := bb.GetID()
// 	for _, agent := range fellowBikers {
// 		votes[agent.GetID()] = 0.0
// 		avgOp := bb.GetAverageOpinionOfAgent(agent.GetID())
// 		if agent.GetID() != bb.GetID() {
// 			val, ok := bb.opinions[agent.GetID()]
// 			if ok {
// 				avgOp = (avgOp + 3*val.opinion) / 4
// 			}
// 		}
// 		if avgOp > maxOpinion {
// 			maxOpinion = avgOp
// 			leaderVote = agent.GetID()
// 		}
// 	}
// 	votes[leaderVote] = 1.0
// 	return votes
// }

//--------------------END OF LEADER/DICTATOR VOTING FUNCTIONS------------------

//--------------------DICTATOR FUNCTIONS------------------

// ** called only when the agent is the dictator
func (bb *Biker1) sumBoxCubeScores(colour utils.Colour) float64 {
	cube := 0.
	// for all fellow bikers whose color match
	bikers := bb.GetFellowBikers()
	for _, biker := range bikers {
		if biker.GetColour() == colour {
			// sum cube score
			cube += bb.calculateCubeScoreForAgent(biker)
		}
	}
	return cube
}

// Finds all boxes within our reachable distance
func (bb *Biker1) calculateDictatorBoxScore(currLocation utils.Coordinates, energy float64, n int, E float64) float64 {
	lootBoxes := bb.GetGameState().GetLootBoxes()
	var currDist float64
	var score float64
	D := 0.
	C := 0.
	var distanceFactor float64
	var remainingEnergy float64

	for _, loot := range lootBoxes {
		lootPos := loot.GetPosition()
		currDist = physics.ComputeDistance(currLocation, lootPos)
		if currDist > bb.energyToReachableDistance(energy) {
			continue
		}
		// increase density by 1
		D += 1
		if currDist == 0 {
			// this is the main box: distance factor is one
			distanceFactor = 1
		} else {
			// calculate energy to travel currDist
			remainingEnergy = bb.distanceToEnergy(currDist, energy)
			// distanceFactor = energy travelled / energy before
			distanceFactor = remainingEnergy / energy
		}
		C += bb.sumBoxCubeScores(loot.GetColour()) * distanceFactor
	}

	score = C * (energy / E) * (D / float64(n))
	return score
}

func (bb *Biker1) DictateDirection() uuid.UUID {
	// nomination
	// for each agent on our bike	// get the closest box of their colour

	// check distance to box
	// Get remaining energy if we went to that box
	// Get all boxes reachable from nominated box and the remaining energy
	// Find density of boxes around that box
	// Evaluate box based on cube, density and distance
	// nominaton score = (densitty/n) * (remaining energy/total energy) * (Cube1 + sum((Er/Etot)*Cubei))

	lootBoxes := bb.GetGameState().GetLootBoxes()
	currLocation := bb.GetLocation()
	fellowBikers := bb.GetFellowBikers()
	initialEnergy := bb.GetEnergyLevel()

	maxScore := 0.
	vote := uuid.Nil

	for _, agent := range fellowBikers {
		// for each agent on our bike, get the closest box of their colour
		nearestColourBox, distanceToNearestBox := bb.nearestLootColour(agent.GetLocation(), agent.GetColour())
		// get remaining energy if we went to that box
		remainingEnergy := bb.distanceToEnergy(distanceToNearestBox, initialEnergy)
		// get score for this box
		boxScore := bb.calculateDictatorBoxScore(currLocation, remainingEnergy, len(lootBoxes), initialEnergy)

		if boxScore >= maxScore {
			maxScore = boxScore
			vote = nearestColourBox
		}
	}

	return vote
}

// ** decide which agents to kick out (dictator)
// func (bb *Biker1) DecideKickOut() []uuid.UUID {

// 	// TODO: make more sophisticated
// 	tmp := []uuid.UUID{}
// 	tmp = append(tmp, bb.lowestOpinionKick())
// 	return tmp
// }

// ** decide the allocation (dictator)
// func (bb *Biker1) DecideDictatorAllocation() voting.IdVoteMap {
// 	return bb.DecideAllocation()
// }

//--------------------END OF DICTATOR FUNCTIONS------------------

// --------------------LEADER FUNCTIONS------------------
func (bb *Biker1) DecideWeights(action utils.Action) map[uuid.UUID]float64 {
	// decides the weights of other peoples votes
	// Leadership democracy
	// takes in proposed action as a parameter
	// only run for the leader after everyone's proposeDirection is run
	// assigns vector of weights to everyone's proposals, 0.5 is neutral

	//consider adding weights for agents with low points
	fellowBikers := bb.GetFellowBikers()
	weights := map[uuid.UUID]float64{}

	for _, agent := range fellowBikers {
		_, ok := bb.opinions[agent.GetID()]
		if !ok {
			weights[agent.GetID()] = 0.5
		} else {
			weights[agent.GetID()] = bb.calculateCubeScoreForAgent(agent)
		}
	}
	return weights
}

//--------------------END OF LEADER FUNCTIONS------------------
//--------------------END OF GOVERMENT CHOICE FUNCTIONS------------------

// ---------------------SOCIAL FUNCTIONS------------------------
// get reputation value of all other agents
// func (bb *Biker1) GetReputation() map[uuid.UUID]float64 {
// 	reputation := map[uuid.UUID]float64{}
// 	for agent, opinion := range bb.opinions {
// 		reputation[agent] = opinion.opinion
// 	}
// 	return reputation
// }

// query for reputation value of specific agent with UUID
// func (bb *Biker1) QueryReputation(agent uuid.UUID) float64 {
// 	val, ok := bb.opinions[agent]
// 	if ok {
// 		return val.opinion
// 	} else {
// 		return 0.5
// 	}
// }

// set reputation value of specific agent with UUID
// func (bb *Biker1) SetReputation(agent uuid.UUID, reputation float64) {
// 	bb.opinions[agent] = Opinion{
// 		effort:   bb.opinions[agent].effort,
// 		trust:    bb.opinions[agent].trust,
// 		fairness: bb.opinions[agent].fairness,
// 		opinion:  reputation,
// 	}
// }

//---------------------END OF SOCIAL FUNCTIONS------------------------

// -------------------INSTANTIATION FUNCTIONS----------------------------
func GetBiker1(colour utils.Colour, id uuid.UUID) *Biker1 {
	fmt.Printf("Creating Biker1 with id %v\n", id)
	return &Biker1{
		BaseBiker: obj.GetBaseBiker(colour, id),
	}
}

// -------------------END OF INSTANTIATION FUNCTIONS---------------------
