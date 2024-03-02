package cardsagainstdiscord

func init() {
	pack := &CardPack{
		Name:        "F.R.I.E.N.D.S.",
		Description: "F.R.I.E.N.D.S. TV Show Themed",
		Prompts: []*PromptCard{
			&PromptCard{Prompt: `I like it. Whats not to like? %s? Good. %s? Good. %s? Good.`},
			&PromptCard{Prompt: `Yes Fran. I know what time it is, but I'm looking at %s and I'm not happy!`},
			&PromptCard{Prompt: `According to Chandler, this phenomenon scares the bejesus out of him`},
			&PromptCard{Prompt: `Hello? Vega, yeah we would like some alcohol, and you know what else? We would like some more %s.`},
			&PromptCard{Prompt: `What makes me feel like a perfect arse?`},
			&PromptCard{Prompt: `Stop it! You're killing me! I think I just moved on to phase four: %s!`},
			&PromptCard{Prompt: `The question should be, what is *not* so great about %s. Ok? And the answer would be nothing!`},
			&PromptCard{Prompt: `WHAT ARE YOU DOING TO MY SISTER?`},
			&PromptCard{Prompt: `I'll be %s. When the rain starts to fall.`},
			&PromptCard{Prompt: `I didn't have to, because I was wearing my I heart %s sandwich board and ringing my bell. `},
			&PromptCard{Prompt: `I think Chandler would have achieved phase 3 faster if his friends had tried %s. `},
			&PromptCard{Prompt: `MORRRNING'S HEEEERE, %s is here.`},
			&PromptCard{Prompt: `Home is never far away, home is %s.`},
			&PromptCard{Prompt: `If you can't talk dirty dirty to me, how are you gonna talk dirty to her? Now tell me you wanna caress %s!`},
			&PromptCard{Prompt: `I think Chandler would have achieved phase 3 faster if his friends had tried %s.`},
			&PromptCard{Prompt: `You start with half a dozen European cities, throw in thirty euphemisms for male genitalia, and bam! You have got yourself %s.`},
			&PromptCard{Prompt: `You're a strong, confident woman, who does not need %s.`},
			&PromptCard{Prompt: `Why am I in the "friend zone?"`},
			&PromptCard{Prompt: `Well, Joey. I wrote a song today. It's called %s.`},
			&PromptCard{Prompt: `I need a couch that says, kids welcome here, but that also says %s.`},
			&PromptCard{Prompt: `What isn't a Pheobe song, but could be?`},
			&PromptCard{Prompt: `What makes me feel all floopy?`},
			&PromptCard{Prompt: `I'm out of a thousand dollars, I'm all scratched up and I am stuck with %s.`},
			&PromptCard{Prompt: `Eight and a half hours of aptitude tests, and what do I learn? "You're ideally suited for %s."`},
			&PromptCard{Prompt: `Really, though, how much dirtier can it get?`},
			&PromptCard{Prompt: `According to your bio, You've done quite a bit of work before Days of Our Lives. Anything you're particularly proud of?`},
			&PromptCard{Prompt: `So no one told you that your life was gonna be this way, your job's a joke, you're %s, your love life's %s.`},
			&PromptCard{Prompt: `I needed a plan. A plan to get over my man. And what's the opposite of man?`},
			&PromptCard{Prompt: `What's in Rachel's bag?`},
			&PromptCard{Prompt: `And so, I'm going to get on this spaceship and I'm going to go to Blargon 7 in search of %s.`},
			&PromptCard{Prompt: `Hey, Janice! It's me. Um, yeah, I just want to apologize in advance for %s.`},
			&PromptCard{Prompt: `You go past the mud hut, through the rainbow ring, to get to the golden monkey, you yank his tail and BOOM - you're %s!`},
			&PromptCard{Prompt: `What's the "ick factor" that ended my last relationship?`},
			&PromptCard{Prompt: `What would make this the worse Thanksgiving ever?`},
			&PromptCard{Prompt: `Getting over %s was the hardest thing I've ever had to do.`},
			&PromptCard{Prompt: `What is Chandler Bing's job?`},
			&PromptCard{Prompt: `You know, Chandler, in some cultures, %s is considered a mark of virility.`},
			&PromptCard{Prompt: `What were Ross and Rachel on a break from? `},
			&PromptCard{Prompt: `%s: the new erotic novel by Nora Tyler Bing`},
			&PromptCard{Prompt: `What's a selfless good deed?`},
			&PromptCard{Prompt: `Joey had an imaginary friend. What was his name? `},
			&PromptCard{Prompt: `Ew ew ew! Ugly naked guy got %s!`},
			&PromptCard{Prompt: `What's the least fun game ever?`},
			&PromptCard{Prompt: `Why didn't it work out between Rachel and Joey?`},
			&PromptCard{Prompt: `On second thought, %s would be perfection.`},
			&PromptCard{Prompt: `How would you like to come with me to the Museum of Natural History after everyone has left, just the two of us, and you can touch %s?`},
			&PromptCard{Prompt: `%s is a pretty good way to "own the room."`},
			&PromptCard{Prompt: `What's a great way to say "I secretly love you, roommate's girlfriend"?`},
			&PromptCard{Prompt: ` Until %s came along, there was no snap in my turtle for two years.`},
			&PromptCard{Prompt: `Last episode: The one where %s learns the truth about %s."`},
			&PromptCard{Prompt: ` %s: it's like Mardi Gras, without the paper machine heads.`},
			&PromptCard{Prompt: `I haven't done any of the things I wanted to do by the time I was 31! Like %s, and %s!`},
			&PromptCard{Prompt: `Don't you put words in people's mouth. You put %s in people's mouths.`},
			&PromptCard{Prompt: `What's like a sneeze only better?`},
			&PromptCard{Prompt: `What are Chandler and Monica hiding?`},
			&PromptCard{Prompt: `Okay, but don't touch %s because your fingers have destructive oils.`},
			&PromptCard{Prompt: `Why do you have to break up with her? Be a man. Just stop %s!`},
			&PromptCard{Prompt: `When I'm doing something exciting, and I don't wanna get too excited, I just try and think of other things. Like %s and %s.`},
			&PromptCard{Prompt: `In the unaired episode, "The One with %s," it's revealed that smelly cat's overwhelming scent was due to a steady diet of %s.`},
			&PromptCard{Prompt: `What makes me feel warm in my hollow tin chest?`},
			&PromptCard{Prompt: `Now close your eyes and think of your happy place. Tell me your happy place.`},
			&PromptCard{Prompt: `What's the appropriate response to "I love you"?`},
			&PromptCard{Prompt: `Pressing my third nipple opens the delivery entrance to the magical land of %s.`},
			&PromptCard{Prompt: `I sure wouldn't mind being trapped in an ATM vestibule with %s.`},
			&PromptCard{Prompt: `I hear what you're saying, but at our prices, everyone needs %s.`},
			&PromptCard{Prompt: `From now on, %s is our code word for danger.`},
			&PromptCard{Prompt: `In my experience, if a girl says yes to being taped, she doesn't say no to %s.`},
			&PromptCard{Prompt: `What should I keep a copy of, in a fireproof box at least a hundred yards away from the original?`},
			&PromptCard{Prompt: `What the hell did the damn duck get into now?`},
			&PromptCard{Prompt: `What's "the perfect way to say goodbye"?`},
			&PromptCard{Prompt: `Monica is going to RUE THE DAY she put me in charge of %s!`},
			&PromptCard{Prompt: `What beats rock, paper, and scissors?`},
			&PromptCard{Prompt: `What's "pulling a Monica"?`},
			&PromptCard{Prompt: `I Ross, take thee %s.`},
			&PromptCard{Prompt: `What would liven up Monica's party?`},
		},
		Responses: []ResponseCard{
			`A PHD (pretty huge dick)`,
			`The night of five times`,
			`Faking a British accent`,
			`A really big tongue`,
			`An interplanetary courtship ritual`,
			`The left phalange`,
			`That thing where you act all mean and distant until they break up with you`,
			`%blank`,
			`Kissing a male employee to show the office you aren't homophobic`,
			`Monica Faloola Geller`,
			`Wordless sound poems`,
			`Being Pennsylvania Dutch`,
			`Carrying a little holiday weight`,
			`This generation’s Milton Berle`,
			`Lesbian Lover Day`,
			`Round food for every mood`,
			`Checking out the Chan-Chan Man`,
			`Nipples that you can see through a shirt`,
			`A little corn envelope`,
			`A small pornographic sketch`,
			`The Algonquin kid’s table`,
			`Thursday, the third day`,
			`Crawling up my own ass and dying`,
			`The porn king of the west village`,
			`Nippular areas`,
			`A gay ice dancer`,
			`A cat that’s possessed by the spirit of my dead mom`,
			`The love that Angela has for her cats`,
			`Practicing the art of seduction`,
			`Being a mento for kids`,
			`Goat cheese, watercress, and pancheta`,
			`Getting an erection on the massage table`,
			`A stunning entertainment center with fine Italian craftsmanship`,
			`Paper! Snow! A ghost!`,
			`The days of yore`,
			`Spackleback Harry`,
			`Ugly naked guy`,
			`A pile of garbage`,
			`The WENUS`,
			`The old hug and roll`,
			`Three divorces`,
			`Like a sneeze only better`,
			`Homosexual hair`,
			`Having and giving and sharing and receiving`,
			`Grandma’s chicken salad`,
			`One of those signs that say “we don’t swim in your toilet so don’t pee in your pool”`,
			`A big dull dud`,
			`Just your run of the mill third nipple`,
			`Enticing me with your nakedness`,
			`Smell the fart acting`,
			`Drinking of my pool of inner power`,
			`Being hopeless and awkward and desperate for love`,
			`The meat sweats`,
			`Crazy underwear creeping up my butt`,
			`Vulva`,
			`A ‘take notice’ walk`,
			`Flame retardant boobs`,
			`Five celebrities I can sleep with`,
			`The lesbian sandwich museum`,
			`Pulling on your testicles so hard we have to take you to the eme`,
			`The metaphorical tunnel`,
			`Hugsy the bedtime penguin pal`,
			`Hiking in the foothills of Mount Tibidabo`,
			`Juice squeezed from a person`,
			`A midnight mystery kisser`,
			`Stevie the TV`,
			`A rabbit dog feasting on your danglers`,
			`Flame boy`,
			`A thousand regular cats`,
			`Sexy phlegm`,
			`Free porn`,
			`Dessert stealers`,
			`Erotic novels for children`,
			`Smelly cat`,
			`Ms. Chananadler Bong`,
			`Throwing wet paper towels`,
			`A monkey that has reaquached sexual maturity`,
			`A dirty movie and a bag of mashuga nuts`,
			`Showing brain`,
			`Riding the alimony pony`,
			`A fake work laugh`,
			`Old lady underpants`,
			`The most elaborate filth you have ever heard `,
			`A deer just outside eating fruit from the orchard`,
			`Apartment pants`,
			`A thick endometrial layer`,
			`Stupid onion tartlet`,
			`A moo point`,
			`Smoking three big fat cartons in two days`,
			`Emotional knapsack`,
			`A very serious nougat deficiency`,
			`Strip happy days`,
			`London, baby!`,
			`A condom I’ve had in my wallet since I was twelve`,
			`Toilet seat covers`,
			`Kenny the copy guy`,
			`Yemen`,
			`Getting totally acrimonious`,
			`Getting totally acrimonious`,
			`The scary pigeon from the balcony`,
			`Masturbating in parked car behind a Taco Bell`,
			`Making a bachelorette party stripper cry`,
			`Green eggs and ham discussion group`,
			`Getting all judgmental condescending and pedantic`,
			`free crab cakes`,
			`A lot of grapes`,
			`A violent igneous rock formation`,
			`Having your brother’s babies`,
			`Bursting with Yoo-Hoo`,
			`A real up-and-comer in Human Resources`,
			`The first black man to fly solo across the Atlantic`,
			`Plutonic, same-sex napping`,
			`Software that facilitates inter-business networking e-solutions`,
			`Unagi, a state of total awareness`,
			`Statistical analysis and data reconfiguration`,
			`Lady fingers, jam, custard, raspberries, meat sautéed with peas and onions, bananas and whipped cream`,
			`Trying to divide 232 by 13`,
			`A buttmunch`,
			`Little monkey raisins`,
			`Fajitas`,
			`Portuguese people`,
			`A balloon full of cocaine stuffed up me bum`,
			`Rattling the headboard like a sailor on leave`,
			`Early colonial bird merchants`,
			`Giving out a sexy professor vibe`,
			`Kicking your snooty ass all the way to New Glockenshire`,
			`The last man on earth in a nuclear holocaust`,
			`Counting Mississippily`,
			`Gross mascara goop`,
			`Drinking the fat`,
			`Weird sex stuff from Maxim`,
			`Having a frenaissance`,
			`A flabby gut and saggy man breasts`,
			`The physical act of love`,
			`Macaroni and cheese with cut-up dogs`,
			`Pulling you own arm off just to have something to throw`,
			`The processor who seduces his coworkers’ wives for sport and then laughs about it the next day at the water cooler`,
			`Throbbing pens`,
			`Being sort of insane from the syphilis`,
			`Exchanging dollars for Vermont money`,
			`Fiquackding ninja stars and almost getting your arm broken by a hooker`,
			`Rock paper scissors for who has to tell the whore to leave`,
			`A guy that goes down for two years at a time`,
			`%blank`,
			`A Play date with a stripper`,
			`Drinking a gallon of milk in ten seconds`,
			`Visiting a town a little south of throw-up`,
			`Definite “cupping”`,
			`Definite “cupping”`,
			`Kick you in the crotch, spit on your neck fantastic`,
			`A plate of fries for the table`,
			`The Holiday Armadillo`,
			`Ichiban, lipstick for men`,
			`An eyesore from the Liberace House of Crap`,
			`Shitting yourself on Space Mountain`,
			`Offensive novelty rap`,
			`Reading the dirty magazines without taking off the plastic covers`,
			`A “moist-maker”`,
			`Entering a Vanilla Ice look-a-like contest and winning`,
			`Loving kids the appropriate amount, as allowed by law`,
			`An inadvertent steam room lap dance`,
			`Trying on temporary foreskins made from household objects`,
			`A homeless person in a very serious relationship`,
			`The friend zone`,
			`An identical hand twin`,
			`Gleba`,
			`A song about a guy who likes to have sex with women with giant asses`,
			`The Princess Leia fantasy`,
			`Naked Thursdays`,
			`Shark porn`,
			`Being the poster boy for VD`,
			`Animals dressed as humans`,
			`A porn break`,
			`A huge crap weasel`,
			`Feeling my bicep, and maybe more`,
			`Rambling on for eighteen pages, FRONT AND BACK!`,
			`The hermaphrodite cheerleader from Long Island`,
			`Mockolate, the synthetic chocolate substitute`,
			`Peeing on your friend’s foot`,
			`Princess Consuela Banana Hammock`,
			`Crapbag`,
			`Regina Phalange`,
			`A piece of gravy soaked bread`,
			`A giant poking device`,
			`A can of soda with a thumb in it`,
		},
	}

	AddPack(pack)
}
