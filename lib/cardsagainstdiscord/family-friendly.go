package cardsagainstdiscord

func init() {
	pack := &CardPack{
		Name:        "family-friendly",
		Description: "Family friendly version of CAH",
		Prompts: []*PromptCard{
			{Prompt: `Papa, come quickly! There, in the garden! Do you see %s? Tell me you see it, Papa!`},
			{Prompt: `This is gonna be the best sleepover ever. Once Mom goes to bed, it’s time for %s!`},
			{Prompt: `Time to put on my favorite t-shirt, the one that says “I heart %s."`},
			{Prompt: `Never fear, Captain %s is here!`},
			{Prompt: `And over here is Picasso’s most famous painting, “Portrait of %s."`},
			{Prompt: `Coming soon! Batman vs. %s.`},
			{Prompt: `My dad and I enjoy %s together.`},
			{Prompt: `Kids, Dad is trying something new this week. It's called “%s."`},
			{Prompt: `The aliens are here. They want %s.`},
			{Prompt: `I’m sorry, Jordan, but that’s not an acceptable Science Fair project. That’s just %s.`},
			{Prompt: `Class, pay close attention. I will now demonstrate the physics of %s.`},
			{Prompt: `Hey, kids. I’m Sensei Todd. Today, I’m gonna teach you how to defend yourself against %s.`},
			{Prompt: `Oh, no thank you, Mrs Lee. I’ve had plenty of %s for now.`},
			{Prompt: `They all call me “Mr. %s"`},
			{Prompt: `The warm August air was filled with change. Things were different, for Kayla was now %s.`},
			{Prompt: `Hey Riley, I’ll give you five bucks if you try %s.`},
			{Prompt: `MY NAME IS CHUNGO. CHUNGO LOVE %s.`},
			{Prompt: `There’s nothing better than a peanut butter and %s sandwich.`},
			{Prompt: `CHUNGO FEEL SICK. CHUNGO NO LIKE %s ANYMORE.`},
			{Prompt: `Mom!? You have to come pick me up! There’s %s at this party!`},
			{Prompt: `Ladies and gentlemen, I have discovered something amazing. I have discovered %s.`},
			{Prompt: `My name is Peter Parker. I was bitten by a radioactive spider, and now I’m %s.`},
			{Prompt: `Little Miss Muffet Sat on a tuffet eating her curds and %s.`},
			{Prompt: `Police! Arrest this man! He’s %s.`},
			{Prompt: `On the next episode of Dora the Explorer, Dora explores %s.`},
			{Prompt: `Outback Steakhouse. No rules. Just %s.`},
			{Prompt: `Me and my friends don’t play with dolls anymore. We’re into %s now.`},
			{Prompt: `Ew. Grandpa smells like %s.`},
			{Prompt: `When I pooped, what came out of my butt?`},
			{Prompt: `Alright, which one of you little turds is responsible for %s?!`},
			{Prompt: `What’s about to take this school dance to the next level?`},
			{Prompt: `We’re not supposed to go in the attic. My parents keep %s in there.`},
			{Prompt: `I’m not like other children. Toys bore me, and I don’t care for sweets. I prefer %s.`},
			{Prompt: `I’m sorry, Mrs. Sanchez, but I couldn’t finish my homework because of %s.`},
			{Prompt: `What’s all fun and games until somebody gets hurt?`},
			{Prompt: `Well, look what we have here! A big fancy man walkin’ like he’s %s.`},
			{Prompt: `I have invented a new sport. I call it %s ball.`},
			{Prompt: `CHUNGO ANGRY. CHUNGO DESTROY %s.`},
			{Prompt: `Oh Dark Lord, we show our devotion with a humble offering of %s!`},
			{Prompt: `Young lady, we do not allow %s at the dinner table.`},
			{Prompt: `ENOUGH! I will not let %s tear this family apart!`},
			{Prompt: `Hey, check out my band! We’re called “Rage Against %s."`},
			{Prompt: `At school, I’m just a Mandy. But at summer camp, I’m “%s Mandy."`},
			{Prompt: `Thanks for watching! If you want to see more videos of %s, smash that subscribe.`},
			{Prompt: `Welcome! We’re glad you’re here. Now sit back, relax, and enjoy %s.`},
			{Prompt: `Rub a dub dub, %s in a tub!`},
			{Prompt: `Moms love %s.`},
			{Prompt: `James Bond will return in “No Time for %s."`},
			{Prompt: `We’re off to see the wizard, the wizard of wonderful %s!`},
			{Prompt: `My favorite book is The Amazing Adventures of %s.`},
			{Prompt: `Madam President, we’ve run out of time. The only option is %s.`},
			{Prompt: `Where do babies come from?`},
			{Prompt: `It’s BIG. It’s SCARY. It’s %s!`},
			{Prompt: `Disney proudly presents: “%s on Ice."`},
			{Prompt: `You don’t love me, Sam. All you care about is %s.`},
			{Prompt: `Hey guys. I just want to tell all my followers who are struggling with %s: it DOES get better.`},
			{Prompt: `I lost my arm in a %s accident.`},
			{Prompt: `Run, run, as fast as you can. You can’t catch me, I’m %s!`},
			{Prompt: `What really killed the dinosaurs?`},
			{Prompt: `Did you know that Benjamin Franklin invented %s?`},
			{Prompt: `Whoa there, partner! Looks like %s spooked my horse.`},
			{Prompt: `Isn’t this great, honey? Just you, me, the kids, and %s.`},
			{Prompt: `New from Mattel, it’s %s Barbie!`},
			{Prompt: `Beep beep! %s coming through!`},
			{Prompt: `One nation, under God, indivisible, with liberty and %s for all.`},
			{Prompt: `Princess Marigold, the kingdom is in danger! You must stop %s.`},
			{Prompt: `Foolish child! Did you think you could escape from %s?`},
			{Prompt: `No fair! How come Chloe gets her own phone, and all I get is %s?`},
			{Prompt: `Now in bookstores: Nancy Drew and the Mystery of: %s.`},
			{Prompt: `The easiest way to tell me and my twin apart is that I have a freckle on my cheek and she’s %s.`},
			{Prompt: `Shut up, Becky! At least I’m not %s.`},
			{Prompt: `Girls just wanna have %s.`},
			{Prompt: `Bow before me, for I am the queen of %s!`},
			{Prompt: `New from Hasbro! It’s BUNGO: The Game of %s.`},
			{Prompt: `What killed Old Jim?`},
			{Prompt: `ME HUNGRY. ME WANT %s.`},
			{Prompt: `Coming to theaters this holiday season, “Star Wars: The Rise of %s."`},
			{Prompt: `I don’t really know what my mom’s job is, but I think it has something to do with %s.`},
			{Prompt: `Next from J.K Rowling: Harry Potter and the Chamber of %s.`},
			{Prompt: `Oh, that’s my mom’s friend Carl. He comes over and helps her with %s.`},
			{Prompt: `I do not fight for wealth. I do not fight for glory. I fight for %s!`},
			{Prompt: `Our day at the water park was totally ruined by %s.`},
			{Prompt: `What’s keeping Dad so busy in the garage?`},
			{Prompt: `Guys, stop it! There’s nothing funny about %s.`},
			{Prompt: `Come on, Danny. All the cool kids are doin’ it. Wanna try %s?`},
			{Prompt: `Charmander has evolved into %s!`},
			{Prompt: `Boys? No. %s? Yes!`},
			{Prompt: `Huddle up, Wildcats! They may be bigger. They may be faster. But we’ve got a secret weapon: %s.`},
		},
		Responses: []ResponseCard{
			`A big wet kiss from Great Aunt Sharon`,
			`A cloud that rains diarrhea`,
			`A pirate with two peg arms, two peg legs, and a peg head`,
			`Athena, Goddess of Wisdom`,
			`Baby boomers`,
			`Beautiful Grandma`,
			`Defecating on the neighbor’s lawn`,
			`Eight hours of video games`,
			`Going night-night`,
			`Kissing Mom on the lips`,
			`Me, your dad`,
			`Playing trumpet for the Mayor`,
			`Ratzilla`,
			`Teeny tiny turds`,
			`That there tarantula`,
			`The way Grandpa smells`,
			`Three glasses of red wine`,
			`Whatever Dad does at work`,
			`Getting shot out of a cannon`,
			`Jesus`,
			`Eating pasta out of my pants`,
			`A burrito smoothie`,
			`A bear`,
			`Old people`,
			`My sister’s stupid boyfriend`,
			`Stuffing my underwear with pancakes`,
			`Stinky Martha, the superhero that nobody likes`,
			`My chainsaw`,
			`Diarrhea`,
			`Boogers`,
			`The police`,
			`Sniffing a dog’s butt`,
			`Horrible allergies`,
			`Your face`,
			`A gerbil named “Gerbil”`,
			`Fortis Magnimus, God of Beans`,
			`Rated-R stuff`,
			`Blowing up the Moon`,
			`Getting launched into space`,
			`Likes`,
			`A bird pooping on the president’s head`,
			`Chungo, the talking gorilla`,
			`The way I feel when I see Kyle`,
			`Hot gossip`,
			`Forgetting to put on underwear`,
			`Running full speed into a wall`,
			`The Russians`,
			`A black hole`,
			`Putting my butt on stuff`,
			`Racism, sexism, and homophobia`,
			`Making the bees angry`,
			`My strong, terrifying daughter`,
			`A Democrat`,
			`TikTok`,
			`Cheeto fingers`,
			`A huge honkin’ carrot`,
			`The country of Bolivia`,
			`My parents`,
			`Idiots`,
			`Filling my butt with spaghetti`,
			`Putting an apple in a little boy’s mouth and roasting him for dinner`,
			`Thousands of lasagna`,
			`Climbing into a cow’s butt`,
			`War with Canada`,
			`Mashing a banana into your belly button`,
			`China`,
			`Shrek`,
			`Cavities`,
			`Eating people`,
			`Butts of all shapes and sizes`,
			`A 40-piece Chicken McNuggets`,
			`Climate change`,
			`Barf`,
			`A poop as big as Mom`,
			`A couch that eats children`,
			`Going to hell`,
			`The loose skin at the joint of the elbow known as “the weenus”`,
			`A horse with no legs`,
			`Fat stacks of cash`,
			`Dora the Explorer`,
			`Illegal drugs`,
			`Bulbasaur`,
			`Dad’s meatloaf`,
			`Extra-warm Pepsi`,
			`The old man with the rake who lives down the dark and winding road`,
			`Calling 911`,
			`Spending my parent’s hard-earned money`,
			`Toe jam`,
			`Hot lava`,
			`Butt surgery`,
			`Freeing all the animals from the zoo`,
			`A bountiful harvest of squashes and corns`,
			`Bombs`,
			`Boobies`,
			`Respecting personal boundaries`,
			`Having no idea what’s going on`,
			`John Wilkes Booth`,
			`Getting married`,
			`Politics`,
			`Garbage`,
			`Naked people`,
			`This stupid game`,
			`Biting a rich person`,
			`Screaming at birds`,
			`Getting stuck in the toilet`,
			`Getting crushed by a piano`,
			`Huge pants`,
			`Mom’s spaghetti`,
			`An old, dirty cat with bad breath`,
			`A corn dog`,
			`My followers`,
			`A statue of a naked guy`,
			`A tiny detective who solves tiny crimes`,
			`Joining the army`,
			`Witchcraft`,
			`Taking out my eyeballs`,
			`Sacrificing Uncle Tim`,
			`Moving to Ohio`,
			`A hundred screaming monkeys`,
			`A wise old woman with no teeth and cloudy eyes`,
			`Emotions`,
			`Ham`,
			`Clams`,
			`Spider-Man`,
			`Drinking out of the toilet and eating garbage`,
			`Trying to catch that damn raccoon`,
			`Slapping my huge belly`,
			`The doll that watches me sleep`,
			`Money`,
			`LeBron James`,
			`Father’s forbidden chocolates`,
			`Me`,
			`Salmon`,
			`Screaming the F-word`,
			`A whole thing of cottage cheese`,
			`Crab-walking from the toilet to get more toilet paper`,
			`Mayonnaise`,
			`Total world domination`,
			`Failure`,
			`JoJo Siwa`,
			`Getting kicked in the nuts`,
			`Farting and walking away`,
			`Sharks with legs`,
			`Triangles`,
			`Ninjas`,
			`The floor`,
			`The whole family`,
			`Bench pressing a horse`,
			`Spiders`,
			`Uranus`,
			`A Republican`,
			`Giving wedgies to my haters`,
			`14 cheeseburgers, 6 large fries, and a medium Sprite`,
			`Big Randy`,
			`Blasting my math teacher into the sun`,
			`Dreaming about boys`,
			`Taking a dump in the pool`,
			`Crying in the bathroom`,
			`Happiness`,
			`Poison`,
			`My sister’s hair all over the place`,
			`A scoop of tuna`,
			`Doing karate`,
			`Hades, God of the Underworld`,
			`Lil Nas X`,
			`Dad’s famous poops`,
			`Squirty cheese`,
			`My girlfriend, who goes to another school`,
			`Mowing the stupid lawn`,
			`Slapping that ass`,
			`Questioning authority`,
			`Chugging a gallon of milk and then vomiting a gallon of milk`,
			`Farting a lot today`,
			`Going bald`,
			`Germs`,
			`Nuclear war`,
			`Barfing into a popcorn bucket`,
			`A cursed llama with no eyes`,
			`The wettest fart you ever heard`,
			`Shaving Dad’s back`,
			`Overthrowing the government`,
			`Murdering`,
			`Exploding`,
			`Bleeding`,
			`A long, hot pee`,
			`Throwing up double peace signs with my besties at Starbucks`,
			`A balloon filled with chili`,
			`The dishes`,
			`The freedom of speech`,
			`Egg salad`,
			`Mom’s friend, Donna`,
			`Rich people`,
			`Pirate music`,
			`Being a dinosaur`,
			`A fake kid made out of wood`,
			`Homework`,
			`Batman`,
			`A dead body`,
			`Drinking a whole bottle of ranch`,
			`Destroying the planet`,
			`Living in a pineapple under the sea`,
			`Pizza`,
			`This pumpkin`,
			`Extremely tight underpants`,
			`A nice, warm glass of pee`,
			`Poseidon, Lord of the Sea`,
			`Complaining`,
			`Bursting into flames`,
			`Spawning sheep`,
			`My whole body getting big and strong and beautiful`,
			`Big, juicy pimples`,
			`The humble earthworm`,
			`The garbage man`,
			`A big, and I mean BIG turtle`,
			`Your mom!`,
			`The octopus stuck to my face`,
			`Dying of old age`,
			`Smashing the patriarchy`,
			`Screaming into a can of Pringles`,
			`Grandpa`,
			`Shutting up`,
			`Silence`,
			`My annoying brother`,
			`My future husband`,
			`A hot air balloon powered by fart gas`,
			`Sitting on the toilet and going poop`,
			`A screaming soccer dad`,
			`Cocktail weenies`,
			`The British`,
			`Voldemort`,
			`Science`,
			`Picking my nose and eating it`,
			`Having a really big head`,
			`A dead whale`,
			`Fortnite`,
			`The power of the Dark Side`,
			`Big but cheeks filled with poop`,
			`My friend Steve`,
			`Violence`,
			`Magic: The Gathering`,
			`Eating toenail clippings`,
			`Using balloons as boobies`,
			`My dang kids`,
			`Squeezing lemon into my eye`,
			`Tombus, the talking rhombus`,
			`Uncle Bob`,
			`Outback Steakhouse`,
			`Dwayne “The Rock” Johnson`,
			`A baby with a full mustache`,
			`Getting hit in the face with a soccer ball`,
			`Eating a whole roll of toilet paper`,
			`The bacon`,
			`The first female president of the United States`,
			`Falling off a mountain`,
			`Licking a used band-aid`,
			`Having to pee so bad`,
			`Braiding my armpit hair`,
			`Walking inside of a spider web`,
			`Blasting farts in the powder room`,
			`Cream`,
			`Balls`,
			`Gluing my butt cheeks together`,
			`Total crap`,
			`Doing crimes and going to jail`,
			`A cow`,
			`How school slowly breaks your spirit and drains your will to live`,
			`Cool sunglasses`,
			`This goat, who is my friend`,
			`This terrible winter of 1609`,
			`Having no friends`,
			`A Pokemon named “Jim”`,
			`Literally dying from the smell of a fart`,
			`Aliens`,
			`Getting sucked into a jet engine`,
			`Peeing into everyone’s mouth`,
			`True love’s kiss`,
			`Snot bubbles`,
			`School`,
			`Not breathing`,
			`My big donkey brother`,
			`Dinner`,
			`Peer pressure`,
			`One weird lookin’ toe`,
			`Teaching a chicken to kill`,
			`The huge, stupid moon`,
			`Butthole`,
			`Peeing in my backpack`,
			`You`,
			`The middle finger`,
			`Math`,
			`Smelling like onions`,
			`Bad parenting`,
			`Boys`,
			`A bra`,
			`Pee-pee`,
			`Being fake`,
			`Mom’s butt`,
			`Sitting on a cake`,
			`Being famous on YouTube`,
			`Nasty Cousin Amber`,
			`Hot, fresh doodies`,
			`An owl that hates you`,
			`Some weird guy`,
			`Hogs`,
			`Tongue kissing`,
			`Feet`,
			`A doll that pees real pee!`,
			`Getting a girlfriend`,
			`A turd that just won’t flush`,
			`Abraham Lincoln`,
			`Sadness`,
			`A naked lady in a painting`,
			`Falling in love with a hotdog`,
			`The entire state of Texas`,
			`Covering myself with ketchup and mustard because I am a hot dog`,
			`Going beast mode`,
			`Chunks`,
			`Throwing stuff at other stuff`,
			`Showing everyone my butt`,
			`A long, long snake`,
			`Going around sniffing people’s armpits`,
			`Peeing on my poopy`,
			`Getting my ponytail stuck in my butt`,
			`Evil`,
			`Getting trampled by horses`,
			`Not wearing pants`,
			`The divorce`,
			`Looking into people’s windows`,
			`A big rock`,
			`The babysitter`,
			`Meatballs, meatballs, meatballs!`,
			`Diaper beans`,
			`Sucking at life`,
			`Rubbing lotion on a hairless cat`,
			`The lunch lady`,
			`Finding Waldo`,
			`Cigarettes`,
			`Crossbows`,
			`Goblins`,
			`Peeing in the cat’s litter box`,
			`The bus driver`,
			`Butt hair`,
			`Anime`,
			`Being dead`,
			`A big sad dragon with no friends`,
			`Getting run over by a train`,
			`Having many husbands`,
			`The sweet honking of Karen’s bassoon`,
			`Pooping barf forever`,
			`A big whiny cry-baby`,
			`Spinning and barfing`,
			`Making the bathroom smell`,
			`Pooping in a bag and lighting it on fire`,
			`FOOD FIIIIIIIIIIIIIIIIIGHT!!!`,
			`Seymour Butts`,
			`Some freakin’ privacy`,
			`Coffee`,
			`Space lasers`,
			`Wearing high heels`,
			`Building a ladder of hot dogs to the moon`,
			`Whispering secrets to my best friend, Turkey`,
			`Taking a selfie`,
			`Cat pee`,
			`Elegant party hats`,
			`Practicing kissing`,
			`Ear wax`,
			`Dancing with my son`,
			`Dump cake`,
			`Santa Claus`,
			`Eating a lightning bug to gain its lightning powers`,
			`Knives`,
			`Punching a guy through a wall`,
			`Billie Eilish`,
			`The gluteus maximus`,
			`Slowly turning into cheese`,
			`Stuff`,
			`Nipples`,
			`Steven Universe`,
			`Baby Yoda`,
			`Like a million alligators`,
			`The fifth graders`,
			`Old Jim’s Steamy Butt Sauce`,
			`A Pringle`,
			`Coming back from the dead`,
			`Big, slappy hands`,
			`Licking a goat`,
			`Going to the emergency room`,
			`Never showering`,
			`Burning books`,
			`Beer`,
			`A flamethrower`,
			`Sitting atop a pile of tuna, like some kind of tuna quee`,
			`Naptime`,
			`Falling in love`,
			`Spit`,
			`Drama!`,
			`Girls`,
			`The gym teacher`,
			`Tossed salads and scrambled eggs`,
			`Being French, hoh-hoh-hoh!`,
			`Slappy Spatchy, the game where you slap each other with spatulas`,
			`Thick, nasty burps`,
			`The president`,
			`Freeing a fart from its butt prison`,
			`Guacamole`,
			`A pregnant person`,
			`Having no bones`,
			`A hug`,
			`An order of mozzarella sticks`,
			`Getting naked`,
			`A bunch of dead squirrels on a trampoline`,
			`Famous peanut scientist George Washington Carver`,
			`The Dark Lord`,
			`Fake news`,
			`A super angry cat I found outside`,
			`Grandma panties`,
			`Saving up my boogers for ten years and then building the world’s largest booger`,
			`Being super serious right now`,
			`The baby`,
			`Using my butt as a microwave`,
			`Many wolves`,
			`Mooing`,
			`Swords`,
			`My annoying sister`,
			`Glen’s fabulous body`,
			`One long hair growing out of a mole`,
			`Robbing a bank`,
			`Kevin’s mom`,
			`Getting slapped with a fish`,
			`Gluten`,
			`A butt that eats underwear`,
			`Punching everyone`,
			`Getting scalded in the face with hot beans`,
			`Harry Potter`,
			`Wakanda`,
			`Poo-poo`,
			`How much wood a woodchuck would chuck if a woodchuck could chuck wood`,
			`Chest hair`,
			`Peeing sand`,
			`Feminism`,
			`Lighting stuff on fire`,
			`An invisible giant who takes giant, visible poops`,
			`Locking Mother in the pantry`,
			`All of my teeth falling out`,
			`Snakes`,
			`Lighting stuff on fire`,
			`A cowboy who is half boy, half cow`,
			`Ariana Grande`,
			`Floating through the void of space and time`,
			`Free ice cream, yo`,
			`Turning 40`,
			`The longest tongue in the world`,
			`Puberty`,
			`Farting, barfing, and passing out,`,
			`Screaming and screaming and never waking up`,
			`Getting a skull tattoo`,
			`Josh`,
			`A glorious beard`,
			`Flamin’ Hot Cheetos`,
			`Running away from home`,
			`My father, who is a walrus`,
			`Releasing the falcons!`,
			`Lice`,
			`Saying mean stuff and making people feel bad`,
			`The beautiful potato`,
			`The dentist`,
			`Literally ruining my life`,
			`A killer clown`,
			`Beyonce`,
			`Legs`,
			`Aunt Linda`,
			`Flushing myself down the toilet;`,
			`Blossoming into a beautiful young man`,
			`A truck`,
			`The woman I’m going to marry one day`,
			`One tough mama`,
			`Fire farts`,
			`Wiping my butt`,
			`Only beans`,
			`Unleashing a hell demon that will destroy our world`,
			`Person Milk`,
			`Nunchucks`,
			`Happy daddies with happy sandals`,
			`Twerking`,
			`Stank breath`,
			`Pink eye`,
			`Nothing`,
			`Giggling and farting and slurping milkshakes`,
			`Reading my sister’s diary`,
			`Living in the dumpster`,
			`Pork`,
			`GOOOOOOOOOALLLL!!`,
			`Ice pee`,
			`The ice cream man`,
			`Having a baby`,
			`The government`,
			`Mom’s new haircut`,
			`Obama`,
			`Love`,
			`Shoplifting`,
			`%blank`,
		},
	}

	AddPack(pack)
}
